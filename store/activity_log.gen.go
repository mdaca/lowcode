package store

// This file is auto-generated.
//
// Template:    pkg/codegen/assets/store_base.gen.go.tpl
// Definitions: store/activity_log.yaml
//
// Changes to this file may cause incorrect behavior and will be lost if
// the code is regenerated.

import (
	"context"
	"github.com/cortezaproject/corteza-server/pkg/activitylog"
)

type (
	ActivityLogs interface {
		SearchActivityLogs(ctx context.Context, f activitylog.Filter) (activitylog.ActivitySet, activitylog.Filter, error)
		LookupActivityLogByID(ctx context.Context, id uint64) (*activitylog.Activity, error)

		CreateActivityLog(ctx context.Context, rr ...*activitylog.Activity) error

		UpdateActivityLog(ctx context.Context, rr ...*activitylog.Activity) error

		UpsertActivityLog(ctx context.Context, rr ...*activitylog.Activity) error

		DeleteActivityLog(ctx context.Context, rr ...*activitylog.Activity) error
		DeleteActivityLogByID(ctx context.Context, ID uint64) error

		TruncateActivityLogs(ctx context.Context) error
	}
)

var _ *activitylog.Activity
var _ context.Context

// SearchActivityLogs returns all matching ActivityLogs from store
func SearchActivityLogs(ctx context.Context, s ActivityLogs, f activitylog.Filter) (activitylog.ActivitySet, activitylog.Filter, error) {
	return s.SearchActivityLogs(ctx, f)
}

// LookupActivityLogByID searches for corteza resource activity by ID
// It returns corteza resource activity even if deleted
func LookupActivityLogByID(ctx context.Context, s ActivityLogs, id uint64) (*activitylog.Activity, error) {
	return s.LookupActivityLogByID(ctx, id)
}

// CreateActivityLog creates one or more ActivityLogs in store
func CreateActivityLog(ctx context.Context, s ActivityLogs, rr ...*activitylog.Activity) error {
	return s.CreateActivityLog(ctx, rr...)
}

// UpdateActivityLog updates one or more (existing) ActivityLogs in store
func UpdateActivityLog(ctx context.Context, s ActivityLogs, rr ...*activitylog.Activity) error {
	return s.UpdateActivityLog(ctx, rr...)
}

// UpsertActivityLog creates new or updates existing one or more ActivityLogs in store
func UpsertActivityLog(ctx context.Context, s ActivityLogs, rr ...*activitylog.Activity) error {
	return s.UpsertActivityLog(ctx, rr...)
}

// DeleteActivityLog Deletes one or more ActivityLogs from store
func DeleteActivityLog(ctx context.Context, s ActivityLogs, rr ...*activitylog.Activity) error {
	return s.DeleteActivityLog(ctx, rr...)
}

// DeleteActivityLogByID Deletes ActivityLog from store
func DeleteActivityLogByID(ctx context.Context, s ActivityLogs, ID uint64) error {
	return s.DeleteActivityLogByID(ctx, ID)
}

// TruncateActivityLogs Deletes all ActivityLogs from store
func TruncateActivityLogs(ctx context.Context, s ActivityLogs) error {
	return s.TruncateActivityLogs(ctx)
}
