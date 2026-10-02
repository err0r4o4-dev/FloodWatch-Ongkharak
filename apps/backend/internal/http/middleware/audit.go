package middleware

import (
	"context"
	"net/http"

	"github.com/floodwatch-ongkharak/backend/internal/admin"
	"github.com/google/uuid"
)

// AuditLogger defines the interface for logging audit events
type AuditLogger interface {
	LogAudit(ctx context.Context, log admin.AuditLog) error
}

// AuditContext holds information about the current request for audit logging
type AuditContext struct {
	Action     admin.AuditAction
	TargetType admin.TargetType
	TargetID   string
	Metadata   map[string]interface{}
}

type auditContextKey struct{}

// WithAuditContext adds audit context to the request context
func WithAuditContext(ctx context.Context, auditCtx AuditContext) context.Context {
	return context.WithValue(ctx, auditContextKey{}, auditCtx)
}

// AuditContextFromContext retrieves audit context from request context
func AuditContextFromContext(ctx context.Context) (AuditContext, bool) {
	auditCtx, ok := ctx.Value(auditContextKey{}).(AuditContext)
	return auditCtx, ok
}

// AuditLog creates middleware that logs administrative actions
func AuditLog(logger AuditLogger, action admin.AuditAction, targetType admin.TargetType) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Prepare audit context
			auditCtx := AuditContext{
				Action:     action,
				TargetType: targetType,
				Metadata:   make(map[string]interface{}),
			}

			// Add audit context to request
			ctx := WithAuditContext(r.Context(), auditCtx)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// LogAuditAfterSuccess logs an audit entry after a successful operation
// This should be called explicitly from handlers after successful operations
func LogAuditAfterSuccess(ctx context.Context, logger AuditLogger, targetID string, additionalMetadata map[string]interface{}) error {
	auditCtx, ok := AuditContextFromContext(ctx)
	if !ok {
		// No audit context, skip logging
		return nil
	}

	principal, ok := PrincipalFromContext(ctx)
	if !ok {
		// No principal, use system actor
		return logSystemAudit(ctx, logger, auditCtx, targetID, additionalMetadata)
	}

	// Merge additional metadata
	metadata := auditCtx.Metadata
	for k, v := range additionalMetadata {
		metadata[k] = v
	}

	actorID := principal.UserID
	log := admin.AuditLog{
		ID:         uuid.NewString(),
		Action:     auditCtx.Action,
		ActorID:    &actorID,
		ActorEmail: "", // Will need to be fetched if required
		ActorRole:  admin.ActorRole(principal.Role),
		TargetType: auditCtx.TargetType,
		TargetID:   targetID,
		Metadata:   metadata,
	}

	// Best-effort logging, don't fail the request if audit logging fails
	_ = logger.LogAudit(ctx, log)
	return nil
}

// logSystemAudit logs an audit entry for system-initiated actions
func logSystemAudit(ctx context.Context, logger AuditLogger, auditCtx AuditContext, targetID string, additionalMetadata map[string]interface{}) error {
	metadata := auditCtx.Metadata
	for k, v := range additionalMetadata {
		metadata[k] = v
	}

	log := admin.AuditLog{
		ID:         uuid.NewString(),
		Action:     auditCtx.Action,
		ActorID:    nil,
		ActorEmail: "system",
		ActorRole:  admin.ActorRoleSystem,
		TargetType: auditCtx.TargetType,
		TargetID:   targetID,
		Metadata:   metadata,
	}

	_ = logger.LogAudit(ctx, log)
	return nil
}

// ExtractRequestMetadata extracts common request metadata for audit logging
func ExtractRequestMetadata(r *http.Request) map[string]interface{} {
	metadata := make(map[string]interface{})
	
	// Extract IP address
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		metadata["ip_address"] = xff
	} else {
		metadata["ip_address"] = r.RemoteAddr
	}

	// Extract user agent
	if ua := r.Header.Get("User-Agent"); ua != "" {
		metadata["user_agent"] = ua
	}

	metadata["method"] = r.Method
	metadata["path"] = r.URL.Path

	return metadata
}
