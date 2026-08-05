package service

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"

    "github.com/gin-gonic/gin"
)

type PayoutRequest struct {
    GuruID        string  `json:"guruId"`
    Amount        float64 `json:"amount"`
    AccountNumber string  `json:"accountNumber"`
    BankCode      string  `json:"bankCode"`
    AccountName   string  `json:"accountName"`
}

func (s *Service) handlePayout(c *gin.Context) {
    var req PayoutRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
        return
    }

    if req.Amount < 1000 {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Minimum withdrawal is ₦1000"})
        return
    }

    secretKey := os.Getenv("PAYSTACK_SECRET_KEY")
    if secretKey == "" {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Paystack secret key not configured"})
        return
    }

    // 1. Create Transfer Recipient
    recipientPayload := map[string]interface{}{
        "type":           "nuban",
        "name":           req.AccountName,
        "account_number": req.AccountNumber,
        "bank_code":      req.BankCode,
        "currency":       "NGN",
    }

    recipientCode, err := createPaystackRecipient(secretKey, recipientPayload)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to create recipient: " + err.Error()})
        return
    }

    // 2. Initiate Transfer
    transferPayload := map[string]interface{}{
        "source":    "balance",
        "amount":    int(req.Amount * 100), // Convert to kobo
        "recipient": recipientCode,
        "reason":    fmt.Sprintf("GaruGO Guru Payout - %s", req.GuruID),
        "reference": fmt.Sprintf("GARUGO-WD-%d", time.Now().Unix()),
    }

    transferData, err := initiatePaystackTransfer(secretKey, transferPayload)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Transfer failed: " + err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success":  true,
        "transfer": transferData,
    })
}

// Helper: Create Transfer Recipient
func createPaystackRecipient(secretKey string, payload map[string]interface{}) (string, error) {
    body, _ := json.Marshal(payload)
    req, _ := http.NewRequest("POST", "https://api.paystack.co/transferrecipient", bytes.NewBuffer(body))
    req.Header.Set("Authorization", "Bearer "+secretKey)
    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    bodyBytes, _ := io.ReadAll(resp.Body)
    var result map[string]interface{}
    json.Unmarshal(bodyBytes, &result)

    if resp.StatusCode != 200 && resp.StatusCode != 201 {
        return "", fmt.Errorf("%v", result["message"])
    }

    data := result["data"].(map[string]interface{})
    return data["recipient_code"].(string), nil
}

// Helper: Initiate Transfer
func initiatePaystackTransfer(secretKey string, payload map[string]interface{}) (map[string]interface{}, error) {
    body, _ := json.Marshal(payload)
    req, _ := http.NewRequest("POST", "https://api.paystack.co/transfer", bytes.NewBuffer(body))
    req.Header.Set("Authorization", "Bearer "+secretKey)
    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    bodyBytes, _ := io.ReadAll(resp.Body)
    var result map[string]interface{}
    json.Unmarshal(bodyBytes, &result)

    if resp.StatusCode != 200 {
        return nil, fmt.Errorf("%v", result["message"])
    }

    return result["data"].(map[string]interface{}), nil
}
