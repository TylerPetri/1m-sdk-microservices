package server

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	authv1 "sdk-microservices/gen/api/proto/auth/v1"
	"sdk-microservices/internal/services/auth/jwt"
	"sdk-microservices/internal/services/auth/password"
	"sdk-microservices/internal/services/auth/store"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

type Server struct {
	authv1.UnimplementedAuthServiceServer

	log *zap.Logger
	s   *store.Store
	jwt *jwt.Service

	accessTTL  time.Duration
	refreshTTL time.Duration
}

type Options struct {
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

func New(log *zap.Logger, st *store.Store, jwtSvc *jwt.Service, opt Options) *Server {
	if opt.AccessTTL == 0 {
		opt.AccessTTL = 15 * time.Minute
	}
	if opt.RefreshTTL == 0 {
		opt.RefreshTTL = 7 * 24 * time.Hour
	}
	return &Server{
		log:        log,
		s:          st,
		jwt:        jwtSvc,
		accessTTL:  opt.AccessTTL,
		refreshTTL: opt.RefreshTTL,
	}
}

func (s *Server) Register(ctx context.Context, req *authv1.RegisterRequest) (*authv1.RegisterResponse, error) {
	email := strings.TrimSpace(strings.ToLower(req.GetEmail()))
	pw := req.GetPassword()

	if !emailRe.MatchString(email) {
		return nil, status.Error(codes.InvalidArgument, "invalid email")
	}
	if len(pw) < 12 {
		return nil, status.Error(codes.InvalidArgument, "password must be at least 12 characters")
	}

	hash, err := password.Hash(pw)
	if err != nil {
		s.log.Error("hash password", zap.Error(err))
		return nil, status.Error(codes.Internal, "internal error")
	}

	u, err := s.s.CreateUser(ctx, email, hash)
	if err != nil {
		// Postgres unique violation text varies; keep it simple.
		if strings.Contains(strings.ToLower(err.Error()), "unique") || strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return nil, status.Error(codes.AlreadyExists, "email already registered")
		}
		s.log.Error("create user", zap.Error(err))
		return nil, status.Error(codes.Internal, "internal error")
	}

	return &authv1.RegisterResponse{UserId: u.ID}, nil
}

func (s *Server) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	email := strings.TrimSpace(strings.ToLower(req.GetEmail()))
	pw := req.GetPassword()

	if !emailRe.MatchString(email) {
		return nil, status.Error(codes.InvalidArgument, "invalid email")
	}
	if pw == "" {
		return nil, status.Error(codes.InvalidArgument, "password required")
	}

	u, err := s.s.GetUserByEmail(ctx, email)
	if err != nil {
		// Avoid user enumeration.
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	if err := password.Verify(pw, u.PasswordHash); err != nil {
		if errors.Is(err, password.ErrMismatch) {
			return nil, status.Error(codes.Unauthenticated, "invalid credentials")
		}
		s.log.Error("verify password", zap.Error(err))
		return nil, status.Error(codes.Internal, "internal error")
	}

	access, exp, err := s.jwt.NewAccessToken(u.ID, u.Email, s.accessTTL)
	if err != nil {
		s.log.Error("issue access token", zap.Error(err))
		return nil, status.Error(codes.Internal, "internal error")
	}
	refresh, _, err := s.jwt.NewRefreshToken(u.ID, u.Email, s.refreshTTL)
	if err != nil {
		s.log.Error("issue refresh token", zap.Error(err))
		return nil, status.Error(codes.Internal, "internal error")
	}

	return &authv1.LoginResponse{
		UserId:                 u.ID,
		AccessToken:            access,
		RefreshToken:           refresh,
		AccessExpiresInSeconds: int64(time.Until(exp).Seconds()),
	}, nil
}

func (s *Server) Validate(ctx context.Context, req *authv1.ValidateRequest) (*authv1.ValidateResponse, error) {
	tok := strings.TrimSpace(req.GetAccessToken())
	if tok == "" {
		return nil, status.Error(codes.InvalidArgument, "access_token required")
	}

	claims, err := s.jwt.Parse(tok)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}

	return &authv1.ValidateResponse{
		UserId: claims.Subject,
		Email:  claims.Email,
	}, nil
}
