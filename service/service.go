package service

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/signal"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/joho/godotenv"
)

// Service represents the main application service.
type Service struct {
    Server         *http.Server
    Sigint         chan os.Signal
    appID          string
    appCertificate string
    allowOrigin    string
}

// Stop service safely
func (s *Service) Stop() {
    signal.Notify(s.Sigint, os.Interrupt)
    <-s.Sigint

    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    cancel()
    err := s.Server.Shutdown(ctx)
    if err != nil {
        log.Println(err)
    }
}

// Start runs the service
func (s *Service) Start() {
    log.Println("Listening to port " + s.Server.Addr)
    if err := s.Server.ListenAndServe(); err != nil {
        panic(err)
    }
}

// NewService returns a Service pointer with all configurations set
func NewService() *Service {
    err := godotenv.Load()
    if err != nil {
        log.Println("Error loading .env file")
    }

    appIDEnv, appIDExists := os.LookupEnv("APP_ID")
    appCertEnv, appCertExists := os.LookupEnv("APP_CERTIFICATE")
    serverPort, serverPortExists := os.LookupEnv("SERVER_PORT")
    corsAllowOrigin, _ := os.LookupEnv("CORS_ALLOW_ORIGIN")

    if !appIDExists || !appCertExists || len(appIDEnv) == 0 || len(appCertEnv) == 0 {
        log.Fatal("FATAL ERROR: ENV not properly configured, check APP_ID and APP_CERTIFICATE")
    }

    if !serverPortExists || len(serverPort) == 0 {
        port, portExists := os.LookupEnv("PORT")
        if portExists && len(port) > 0 {
            serverPort = port
        } else {
            serverPort = "8080"
        }
    }

    s := &Service{
        Sigint: make(chan os.Signal, 1),
        Server: &http.Server{
            Addr: fmt.Sprintf(":%s", serverPort),
        },
        appID:          appIDEnv,
        appCertificate: appCertEnv,
        allowOrigin:    corsAllowOrigin,
    }

    api := gin.Default()

    api.Use(s.nocache())
    api.Use(s.CORSMiddleware())

    // Existing Agora routes
    api.GET("rtc/:channelName/:role/:tokenType/:rtcuid/", s.getRtcToken)
    api.GET("rtm/:rtmuid/", s.getRtmToken)
    api.GET("rte/:channelName/:role/:tokenType/:rtcuid/", s.getRtcRtmToken)
    api.GET("rte/:channelName/:role/:tokenType/:rtcuid/:rtmuid/", s.getRtcRtmToken)
    api.GET("chat/app/", s.getChatToken)
    api.GET("chat/account/:chatid/", s.getChatToken)
    api.GET("/ping", func(c *gin.Context) {
        c.JSON(200, gin.H{"message": "pong"})
    })
    api.POST("/getToken", s.getToken)

    // ========== NEW PAYSTACK PAYOUT ROUTE ==========
    api.POST("/payout", s.handlePayout)

    s.Server.Handler = api
    return s
}
