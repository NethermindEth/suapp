package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"sync"

	builderCapella "github.com/attestantio/go-builder-client/api/capella"
	"github.com/ethereum/go-ethereum/common"
	"github.com/flashbots/go-utils/httplogger"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

const (
	pathGetOPPayload      = "/eth/v1/builder/get_payload/{parent_hash:0x[a-fA-F0-9]+}"
	pathGetBlockFromSuave = "/relay/v1/builder/blocks"
)

var (
	errServerAlreadyRunning = errors.New("server already running")
	errBidNotFound          = errors.New("bid not found")
	errInvalidHash          = errors.New("invalid hash")
)

// BoostService - the mev-boost service
type BoostService struct {
	listenAddr   string
	log          *logrus.Entry
	srv          *http.Server
	payloadCache map[common.Hash]builderCapella.SubmitBlockRequest
	cacheLock    sync.Mutex
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
	r.HandleFunc(pathGetBlockFromSuave, m.handleGetBlockFromSuave).Methods(http.MethodPost)

	r.Use(mux.CORSMethodMiddleware(r))
	loggedRouter := httplogger.LoggingMiddlewareLogrus(m.log, r)
	return loggedRouter
}

func (m *BoostService) handleOPGetPayload(w http.ResponseWriter, req *http.Request) {
	log := m.log.WithField("method", "getPayload")
	log.Debug("getPayload request starts")

	vars := mux.Vars(req)
	parentHashHex := vars["parent_hash"]

	if len(parentHashHex) != 66 {
		m.respondError(w, http.StatusBadRequest, errInvalidHash.Error())
		return
	}

	var (
		bid builderCapella.SubmitBlockRequest
		ok  = false
	)
	m.cacheLock.Lock()
	bid, ok = m.payloadCache[common.HexToHash(parentHashHex)]
	m.cacheLock.Unlock()

	if !ok {
		m.respondError(w, http.StatusNotFound, errBidNotFound.Error())
		return
	}

	m.respondOK(w, translateResponse(bid))
}

func (m *BoostService) handleGetBlockFromSuave(w http.ResponseWriter, req *http.Request) {
	log := m.log.WithField("method", "getBlockFromSuave")
	log.Debug("getBlockFromSuave request starts")

	resp := builderCapella.SubmitBlockRequest{}
	defer req.Body.Close()
	if err := json.NewDecoder(req.Body).Decode(&resp); err != nil {
		m.respondError(w, http.StatusBadRequest, err.Error())
	}

	m.cacheLock.Lock()
	defer m.cacheLock.Unlock()
	m.payloadCache[common.Hash(resp.ExecutionPayload.ParentHash)] = resp

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
