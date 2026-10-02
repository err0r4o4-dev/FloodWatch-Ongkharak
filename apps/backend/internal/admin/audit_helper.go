package admin

import (
	"context"
	"net"

	"github.com/google/uuid"
)

// AuditHelper provides convenient methods for creating audit logs
type AuditHelper struct {
	repository Repository
}

// NewAuditHelper creates a new audit helper
func NewAuditHelper(repository Repository) *AuditHelper {
	return &AuditHelper{repository: repository}
}

// LogAuthSuccess logs a successful authentication attempt
func (h *AuditHelper) LogAuthSuccess(ctx context.Context, userID, email string, role ActorRole, ipAddress, userAgent string) {
	_ = h.repository.CreateAuditLog(ctx, AuditLog{
		ID:         uuid.NewString(),
		Action:     ActionLoginSuccess,
		ActorID:    &userID,
		ActorEmail: email,
		ActorRole:  role,
		TargetType: TargetAuthSession,
		TargetID:   userID,
		Metadata:   make(map[string]interface{}),
		IPAddress:  stringPtr(ipAddress),
		UserAgent:  stringPtr(userAgent),
	})
}

// LogAuthFailure logs a failed authentication attempt
func (h *AuditHelper) LogAuthFailure(ctx context.Context, email, ipAddress, userAgent, reason string) {
	_ = h.repository.CreateAuditLog(ctx, AuditLog{
		ID:         uuid.NewString(),
		Action:     ActionLoginFailed,
		ActorID:    nil,
		ActorEmail: email,
		ActorRole:  ActorRoleUser,
		TargetType: TargetAuthSession,
		TargetID:   email,
		Metadata: map[string]interface{}{
			"reason": reason,
		},
		IPAddress: stringPtr(ipAddress),
		UserAgent: stringPtr(userAgent),
	})
}

// LogLogout logs a logout action
func (h *AuditHelper) LogLogout(ctx context.Context, userID, email string, role ActorRole, ipAddress, userAgent string) {
	_ = h.repository.CreateAuditLog(ctx, AuditLog{
		ID:         uuid.NewString(),
		Action:     ActionLogout,
		ActorID:    &userID,
		ActorEmail: email,
		ActorRole:  role,
		TargetType: TargetAuthSession,
		TargetID:   userID,
		Metadata:   make(map[string]interface{}),
		IPAddress:  stringPtr(ipAddress),
		UserAgent:  stringPtr(userAgent),
	})
}

// LogUserCreated logs user registration
func (h *AuditHelper) LogUserCreated(ctx context.Context, userID, email string, role ActorRole, ipAddress, userAgent string) {
	_ = h.repository.CreateAuditLog(ctx, AuditLog{
		ID:         uuid.NewString(),
		Action:     ActionUserCreated,
		ActorID:    &userID,
		ActorEmail: email,
		ActorRole:  role,
		TargetType: TargetUser,
		TargetID:   userID,
		Metadata:   make(map[string]interface{}),
		IPAddress:  stringPtr(ipAddress),
		UserAgent:  stringPtr(userAgent),
	})
}

// LogReportReviewed logs when an admin reviews a community report
func (h *AuditHelper) LogReportReviewed(ctx context.Context, reportID string, actorID, actorEmail string, actorRole ActorRole, oldStatus, newStatus string) {
	_ = h.repository.CreateAuditLog(ctx, AuditLog{
		ID:         uuid.NewString(),
		Action:     ActionReportReviewed,
		ActorID:    &actorID,
		ActorEmail: actorEmail,
		ActorRole:  actorRole,
		TargetType: TargetCommunityReport,
		TargetID:   reportID,
		Metadata: map[string]interface{}{
			"old_status": oldStatus,
			"new_status": newStatus,
		},
	})
}

// LogAlertRuleCreated logs alert rule creation
func (h *AuditHelper) LogAlertRuleCreated(ctx context.Context, ruleID string, actorID, actorEmail string, actorRole ActorRole, ruleType string) {
	_ = h.repository.CreateAuditLog(ctx, AuditLog{
		ID:         uuid.NewString(),
		Action:     ActionAlertRuleCreated,
		ActorID:    &actorID,
		ActorEmail: actorEmail,
		ActorRole:  actorRole,
		TargetType: TargetAlertRule,
		TargetID:   ruleID,
		Metadata: map[string]interface{}{
			"rule_type": ruleType,
		},
	})
}

// LogAlertRuleUpdated logs alert rule modification
func (h *AuditHelper) LogAlertRuleUpdated(ctx context.Context, ruleID string, actorID, actorEmail string, actorRole ActorRole, changes map[string]interface{}) {
	_ = h.repository.CreateAuditLog(ctx, AuditLog{
		ID:         uuid.NewString(),
		Action:     ActionAlertRuleUpdated,
		ActorID:    &actorID,
		ActorEmail: actorEmail,
		ActorRole:  actorRole,
		TargetType: TargetAlertRule,
		TargetID:   ruleID,
		Metadata:   changes,
	})
}

// LogAlertRuleDeleted logs alert rule deletion
func (h *AuditHelper) LogAlertRuleDeleted(ctx context.Context, ruleID string, actorID, actorEmail string, actorRole ActorRole) {
	_ = h.repository.CreateAuditLog(ctx, AuditLog{
		ID:         uuid.NewString(),
		Action:     ActionAlertRuleDeleted,
		ActorID:    &actorID,
		ActorEmail: actorEmail,
		ActorRole:  actorRole,
		TargetType: TargetAlertRule,
		TargetID:   ruleID,
		Metadata:   make(map[string]interface{}),
	})
}

// ExtractIPAddress extracts the client IP address, handling proxies
func ExtractIPAddress(remoteAddr, xForwardedFor string) string {
	if xForwardedFor != "" {
		// Take the first IP from X-Forwarded-For
		if host, _, err := net.SplitHostPort(xForwardedFor); err == nil {
			return host
		}
		return xForwardedFor
	}
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return remoteAddr
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
