package models

import "github.com/google/uuid"

// IdentityProviderOrganizationRule mirrors `models::control_plane::IdentityProviderOrganizationRule`.
type IdentityProviderOrganizationRule struct {
	Name           string    `json:"name"`
	OrganizationID uuid.UUID `json:"organization_id"`
	Workspace      *string   `json:"workspace"`
	Roles          []string  `json:"roles"`
	TenantTier     *string   `json:"tenant_tier"`
}

// IdentityProviderMapping mirrors `models::control_plane::IdentityProviderMapping`.
type IdentityProviderMapping struct {
	ProviderSlug          string                             `json:"provider_slug"`
	DefaultOrganizationID *uuid.UUID                         `json:"default_organization_id"`
	DefaultWorkspace      *string                            `json:"default_workspace"`
	DefaultRoles          []string                           `json:"default_roles"`
	AllowedEmailDomains   []string                           `json:"allowed_email_domains"`
	OrganizationRules     []IdentityProviderOrganizationRule `json:"organization_rules"`
}

// ResourceQuotaSettings mirrors `models::control_plane::ResourceQuotaSettings`.
//
// `usize` fields are widened to uint64 in Go: Rust's usize is platform-sized
// (64-bit on the targets we support) and the JSON wire form is just a number.
type ResourceQuotaSettings struct {
	MaxQueryLimit              uint64 `json:"max_query_limit"`
	MaxDistributedQueryWorkers uint64 `json:"max_distributed_query_workers"`
	MaxPipelineWorkers         uint64 `json:"max_pipeline_workers"`
	MaxRequestBodyBytes        uint64 `json:"max_request_body_bytes"`
	RequestsPerMinute          uint32 `json:"requests_per_minute"`
	MaxStorageGB               uint32 `json:"max_storage_gb"`
	MaxSharedSpaces            uint32 `json:"max_shared_spaces"`
	MaxGuestSessions           uint32 `json:"max_guest_sessions"`
}

// ResourceManagementPolicy mirrors `models::control_plane::ResourceManagementPolicy`.
type ResourceManagementPolicy struct {
	Name                string                `json:"name"`
	TenantTier          string                `json:"tenant_tier"`
	AppliesToOrgIDs     []uuid.UUID           `json:"applies_to_org_ids"`
	AppliesToWorkspaces []string              `json:"applies_to_workspaces"`
	Quota               ResourceQuotaSettings `json:"quota"`
}
