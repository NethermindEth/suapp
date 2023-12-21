package main

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flashbots/go-utils/httplogger"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// UserAgent is a custom string type to avoid confusing url + userAgent parameters in SendHTTPRequest
type UserAgent string

const (
	HeaderKeySlotUID     = "X-MEVBoost-SlotID"
	HeaderKeyVersion     = "X-MEVBoost-Version"
	HeaderKeyForkVersion = "X-MEVBoost-ForkVersion"
	pathGetOPPayload     = "/eth/v1/builder/get_payload/{parent_hash:0x[a-fA-F0-9]+}"
)

var (
	errServerAlreadyRunning = errors.New("server already running")
	errHTTPErrorResponse    = errors.New("HTTP error response")
	errInvalidForkVersion   = errors.New("invalid fork version")
	errInvalidTransaction   = errors.New("invalid transaction")
	errMaxRetriesExceeded   = errors.New("max retries exceeded")
	errInvalidSlot          = errors.New("invalid slot")
	errInvalidHash          = errors.New("invalid hash")
	errInvalidPubkey        = errors.New("invalid pubkey")
)

// BoostService - the mev-boost service
type BoostService struct {
	listenAddr string
	log        *logrus.Entry
	srv        *http.Server
}

func NewBoostService(log *logrus.Entry, listenAddr string) (*BoostService, error) {
	return &BoostService{
		listenAddr: listenAddr,
		log:        log,
	}, nil
}

// StartHTTPServer starts the HTTP server for this boost service instance
func (m *BoostService) StartHTTPServer() error {
	if m.srv != nil {
		return errServerAlreadyRunning
	}

	m.srv = &http.Server{
		Addr:    m.listenAddr,
		Handler: m.getRouter(),

		//ReadTimeout:       time.Duration(config.ServerReadTimeoutMs) * time.Millisecond,
		//ReadHeaderTimeout: time.Duration(config.ServerReadHeaderTimeoutMs) * time.Millisecond,
		//WriteTimeout:      time.Duration(config.ServerWriteTimeoutMs) * time.Millisecond,
		//IdleTimeout:       time.Duration(config.ServerIdleTimeoutMs) * time.Millisecond,
		//
		//MaxHeaderBytes: config.ServerMaxHeaderBytes,
	}

	err := m.srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (m *BoostService) getRouter() http.Handler {
	r := mux.NewRouter()

	r.HandleFunc("/", m.handleRoot)
	r.HandleFunc(pathGetOPPayload, m.handleOPGetPayload).Methods(http.MethodGet)

	r.Use(mux.CORSMethodMiddleware(r))
	loggedRouter := httplogger.LoggingMiddlewareLogrus(m.log, r)
	return loggedRouter
}

func (m *BoostService) handleOPGetPayload(w http.ResponseWriter, req *http.Request) {
	log := m.log.WithField("method", "getPayload")
	log.Debug("getPayload request starts")

	vars := mux.Vars(req)
	parentHashHex := vars["parent_hash"]

	//ua := UserAgent(req.Header.Get("User-Agent"))

	if len(parentHashHex) != 66 {
		m.respondError(w, http.StatusBadRequest, errInvalidHash.Error())
		return
	}

	// Return the bid
	//m.respondOK(w, &result.response.Payload)
	m.respondOK(w, nilResponse)
}

func (m *BoostService) handleRoot(w http.ResponseWriter, req *http.Request) {
	m.respondOK(w, nilResponse)
}

func (m *BoostService) respondError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	resp := httpErrorResp{code, message}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		m.log.WithField("response", resp).WithError(err).Error("Couldn't write error response")
		http.Error(w, "", http.StatusInternalServerError)
	}
}

func (m *BoostService) respondOK(w http.ResponseWriter, response any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		m.log.WithField("response", response).WithError(err).Error("Couldn't write OK response")
		http.Error(w, "", http.StatusInternalServerError)
	}
}

type httpErrorResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

var (
	nilHash     = common.Hash{}
	nilResponse = struct{}{}
)
