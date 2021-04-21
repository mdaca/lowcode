package rdbms

// This file is an auto-generated file
//
// Template:    pkg/codegen/assets/store_rdbms.gen.go.tpl
// Definitions: store/activity_log.yaml
//
// Changes to this file may cause incorrect behavior
// and will be lost if the code is regenerated.

import (
	"context"
	"database/sql"
	"github.com/Masterminds/squirrel"
	"github.com/cortezaproject/corteza-server/pkg/activitylog"
	"github.com/cortezaproject/corteza-server/pkg/errors"
	"github.com/cortezaproject/corteza-server/pkg/filter"
	"github.com/cortezaproject/corteza-server/store"
	"github.com/cortezaproject/corteza-server/store/rdbms/builders"
)

var _ = errors.Is

// SearchActivityLogs returns all matching rows
//
// This function calls convertActivityLogFilter with the given
// activitylog.Filter and expects to receive a working squirrel.SelectBuilder
func (s Store) SearchActivityLogs(ctx context.Context, f activitylog.Filter) (activitylog.ActivitySet, activitylog.Filter, error) {
	var (
		err error
		set []*activitylog.Activity
		q   squirrel.SelectBuilder
	)

	return set, f, func() error {
		q = s.activityLogsSelectBuilder()

		// Paging enabled
		// {search: {enablePaging:true}}
		// Cleanup unwanted cursor values (only relevant is f.PageCursor, next&prev are reset and returned)
		f.PrevPage, f.NextPage = nil, nil

		if f.PageCursor != nil {
			// Page cursor exists so we need to validate it against used sort
			// To cover the case when paging cursor is set but sorting is empty, we collect the sorting instructions
			// from the cursor.
			// This (extracted sorting info) is then returned as part of response
			if f.Sort, err = f.PageCursor.Sort(f.Sort); err != nil {
				return err
			}
		}

		// Make sure results are always sorted at least by primary keys
		if f.Sort.Get("id") == nil {
			f.Sort = append(f.Sort, &filter.SortExpr{
				Column:     "id",
				Descending: f.Sort.LastDescending(),
			})
		}

		// Cloned sorting instructions for the actual sorting
		// Original are passed to the fetchFullPageOfUsers fn used for cursor creation so it MUST keep the initial
		// direction information
		sort := f.Sort.Clone()

		// When cursor for a previous page is used it's marked as reversed
		// This tells us to flip the descending flag on all used sort keys
		if f.PageCursor != nil && f.PageCursor.ROrder {
			sort.Reverse()
		}

		// Apply sorting expr from filter to query
		if q, err = setOrderBy(q, sort, s.sortableActivityLogColumns()); err != nil {
			return err
		}

		set, f.PrevPage, f.NextPage, err = s.fetchFullPageOfActivityLogs(
			ctx,
			q, f.Sort, f.PageCursor,
			f.Limit,
			f.Check,
			func(cur *filter.PagingCursor) squirrel.Sqlizer {
				return builders.CursorCondition(cur, nil)
			},
		)

		if err != nil {
			return err
		}

		f.PageCursor = nil
		return nil
	}()
}

// fetchFullPageOfActivityLogs collects all requested results.
//
// Function applies:
//  - cursor conditions (where ...)
//  - limit
//
// Main responsibility of this function is to perform additional sequential queries in case when not enough results
// are collected due to failed check on a specific row (by check fn).
//
// Function then moves cursor to the last item fetched
func (s Store) fetchFullPageOfActivityLogs(
	ctx context.Context,
	q squirrel.SelectBuilder,
	sort filter.SortExprSet,
	cursor *filter.PagingCursor,
	reqItems uint,
	check func(*activitylog.Activity) (bool, error),
	cursorCond func(*filter.PagingCursor) squirrel.Sqlizer,
) (set []*activitylog.Activity, prev, next *filter.PagingCursor, err error) {
	var (
		aux []*activitylog.Activity

		// When cursor for a previous page is used it's marked as reversed
		// This tells us to flip the descending flag on all used sort keys
		reversedOrder = cursor != nil && cursor.ROrder

		// copy of the select builder
		tryQuery squirrel.SelectBuilder

		// Copy no. of required items to limit
		// Limit will change when doing subsequent queries to fill
		// the set with all required items
		limit = reqItems

		// cursor to prev. page is only calculated when cursor is used
		hasPrev = cursor != nil

		// next cursor is calculated when there are more pages to come
		hasNext bool
	)

	set = make([]*activitylog.Activity, 0, DefaultSliceCapacity)

	for try := 0; try < MaxRefetches; try++ {
		if cursor != nil {
			tryQuery = q.Where(cursorCond(cursor))
		} else {
			tryQuery = q
		}

		if limit > 0 {
			// fetching + 1 so we know if there are more items
			// we can fetch (next-page cursor)
			tryQuery = tryQuery.Limit(uint64(limit + 1))
		}

		if aux, err = s.QueryActivityLogs(ctx, tryQuery, check); err != nil {
			return nil, nil, nil, err
		}

		if len(aux) == 0 {
			// nothing fetched
			break
		}

		// append fetched items
		set = append(set, aux...)

		if reqItems == 0 {
			// no max requested items specified, break out
			break
		}

		collected := uint(len(set))

		if reqItems > collected {
			// not enough items fetched, try again with adjusted limit
			limit = reqItems - collected

			if limit < MinEnsureFetchLimit {
				// In case limit is set very low and we've missed records in the first fetch,
				// make sure next fetch limit is a bit higher
				limit = MinEnsureFetchLimit
			}

			// Update cursor so that it points to the last item fetched
			cursor = s.collectActivityLogCursorValues(set[collected-1], sort...)

			// Copy reverse flag from sorting
			cursor.LThen = sort.Reversed()
			continue
		}

		if reqItems < collected {
			set = set[:reqItems]
			hasNext = true
		}

		break
	}

	collected := len(set)

	if collected == 0 {
		return nil, nil, nil, nil
	}

	if reversedOrder {
		// Fetched set needs to be reversed because we've forced a descending order to get the previous page
		for i, j := 0, collected-1; i < j; i, j = i+1, j-1 {
			set[i], set[j] = set[j], set[i]
		}

		// when in reverse-order rules on what cursor to return change
		hasPrev, hasNext = hasNext, hasPrev
	}

	if hasPrev {
		prev = s.collectActivityLogCursorValues(set[0], sort...)
		prev.ROrder = true
		prev.LThen = !sort.Reversed()
	}

	if hasNext {
		next = s.collectActivityLogCursorValues(set[collected-1], sort...)
		next.LThen = sort.Reversed()
	}

	return set, prev, next, nil
}

// QueryActivityLogs queries the database, converts and checks each row and
// returns collected set
//
// Fn also returns total number of fetched items and last fetched item so that the caller can construct cursor
// for next page of results
func (s Store) QueryActivityLogs(
	ctx context.Context,
	q squirrel.Sqlizer,
	check func(*activitylog.Activity) (bool, error),
) ([]*activitylog.Activity, error) {
	var (
		tmp = make([]*activitylog.Activity, 0, DefaultSliceCapacity)
		set = make([]*activitylog.Activity, 0, DefaultSliceCapacity)
		res *activitylog.Activity

		// Query rows with
		rows, err = s.Query(ctx, q)
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()
	for rows.Next() {
		if err = rows.Err(); err == nil {
			res, err = s.internalActivityLogRowScanner(rows)
		}

		if err != nil {
			return nil, err
		}

		tmp = append(tmp, res)
	}

	for _, res = range tmp {

		// check fn set, call it and see if it passed the test
		// if not, skip the item
		if check != nil {
			if chk, err := check(res); err != nil {
				return nil, err
			} else if !chk {
				continue
			}
		}

		set = append(set, res)
	}

	return set, nil
}

// LookupActivityLogByID searches for corteza resource activity by ID
// It returns corteza resource activity even if deleted
func (s Store) LookupActivityLogByID(ctx context.Context, id uint64) (*activitylog.Activity, error) {
	return s.execLookupActivityLog(ctx, squirrel.Eq{
		s.preprocessColumn("ra.id", ""): store.PreprocessValue(id, ""),
	})
}

// CreateActivityLog creates one or more rows in activity_log table
func (s Store) CreateActivityLog(ctx context.Context, rr ...*activitylog.Activity) (err error) {
	for _, res := range rr {
		err = s.checkActivityLogConstraints(ctx, res)
		if err != nil {
			return err
		}

		err = s.execCreateActivityLogs(ctx, s.internalActivityLogEncoder(res))
		if err != nil {
			return err
		}
	}

	return
}

// UpdateActivityLog updates one or more existing rows in activity_log
func (s Store) UpdateActivityLog(ctx context.Context, rr ...*activitylog.Activity) error {
	return s.partialActivityLogUpdate(ctx, nil, rr...)
}

// partialActivityLogUpdate updates one or more existing rows in activity_log
func (s Store) partialActivityLogUpdate(ctx context.Context, onlyColumns []string, rr ...*activitylog.Activity) (err error) {
	for _, res := range rr {
		err = s.checkActivityLogConstraints(ctx, res)
		if err != nil {
			return err
		}

		err = s.execUpdateActivityLogs(
			ctx,
			squirrel.Eq{
				s.preprocessColumn("ra.id", ""): store.PreprocessValue(res.ID, ""),
			},
			s.internalActivityLogEncoder(res).Skip("id").Only(onlyColumns...))
		if err != nil {
			return err
		}
	}

	return
}

// UpsertActivityLog updates one or more existing rows in activity_log
func (s Store) UpsertActivityLog(ctx context.Context, rr ...*activitylog.Activity) (err error) {
	for _, res := range rr {
		err = s.checkActivityLogConstraints(ctx, res)
		if err != nil {
			return err
		}

		err = s.execUpsertActivityLogs(ctx, s.internalActivityLogEncoder(res))
		if err != nil {
			return err
		}
	}

	return nil
}

// DeleteActivityLog Deletes one or more rows from activity_log table
func (s Store) DeleteActivityLog(ctx context.Context, rr ...*activitylog.Activity) (err error) {
	for _, res := range rr {

		err = s.execDeleteActivityLogs(ctx, squirrel.Eq{
			s.preprocessColumn("ra.id", ""): store.PreprocessValue(res.ID, ""),
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// DeleteActivityLogByID Deletes row from the activity_log table
func (s Store) DeleteActivityLogByID(ctx context.Context, ID uint64) error {
	return s.execDeleteActivityLogs(ctx, squirrel.Eq{
		s.preprocessColumn("ra.id", ""): store.PreprocessValue(ID, ""),
	})
}

// TruncateActivityLogs Deletes all rows from the activity_log table
func (s Store) TruncateActivityLogs(ctx context.Context) error {
	return s.Truncate(ctx, s.activityLogTable())
}

// execLookupActivityLog prepares ActivityLog query and executes it,
// returning activitylog.Activity (or error)
func (s Store) execLookupActivityLog(ctx context.Context, cnd squirrel.Sqlizer) (res *activitylog.Activity, err error) {
	var (
		row rowScanner
	)

	row, err = s.QueryRow(ctx, s.activityLogsSelectBuilder().Where(cnd))
	if err != nil {
		return
	}

	res, err = s.internalActivityLogRowScanner(row)
	if err != nil {
		return
	}

	return res, nil
}

// execCreateActivityLogs updates all matched (by cnd) rows in activity_log with given data
func (s Store) execCreateActivityLogs(ctx context.Context, payload store.Payload) error {
	return s.Exec(ctx, s.InsertBuilder(s.activityLogTable()).SetMap(payload))
}

// execUpdateActivityLogs updates all matched (by cnd) rows in activity_log with given data
func (s Store) execUpdateActivityLogs(ctx context.Context, cnd squirrel.Sqlizer, set store.Payload) error {
	return s.Exec(ctx, s.UpdateBuilder(s.activityLogTable("ra")).Where(cnd).SetMap(set))
}

// execUpsertActivityLogs inserts new or updates matching (by-primary-key) rows in activity_log with given data
func (s Store) execUpsertActivityLogs(ctx context.Context, set store.Payload) error {
	upsert, err := s.config.UpsertBuilder(
		s.config,
		s.activityLogTable(),
		set,
		s.preprocessColumn("id", ""),
	)

	if err != nil {
		return err
	}

	return s.Exec(ctx, upsert)
}

// execDeleteActivityLogs Deletes all matched (by cnd) rows in activity_log with given data
func (s Store) execDeleteActivityLogs(ctx context.Context, cnd squirrel.Sqlizer) error {
	return s.Exec(ctx, s.DeleteBuilder(s.activityLogTable("ra")).Where(cnd))
}

func (s Store) internalActivityLogRowScanner(row rowScanner) (res *activitylog.Activity, err error) {
	res = &activitylog.Activity{}

	if _, has := s.config.RowScanners["activityLog"]; has {
		scanner := s.config.RowScanners["activityLog"].(func(_ rowScanner, _ *activitylog.Activity) error)
		err = scanner(row, res)
	} else {
		err = row.Scan(
			&res.ID,
			&res.ResourceID,
			&res.ResourceType,
			&res.ResourceAction,
			&res.Timestamp,
		)
	}

	if err == sql.ErrNoRows {
		return nil, store.ErrNotFound.Stack(1)
	}

	if err != nil {
		return nil, errors.Store("could not scan activityLog db row: %s", err).Wrap(err)
	} else {
		return res, nil
	}
}

// QueryActivityLogs returns squirrel.SelectBuilder with set table and all columns
func (s Store) activityLogsSelectBuilder() squirrel.SelectBuilder {
	return s.SelectBuilder(s.activityLogTable("ra"), s.activityLogColumns("ra")...)
}

// activityLogTable name of the db table
func (Store) activityLogTable(aa ...string) string {
	var alias string
	if len(aa) > 0 {
		alias = " AS " + aa[0]
	}

	return "activity_log" + alias
}

// ActivityLogColumns returns all defined table columns
//
// With optional string arg, all columns are returned aliased
func (Store) activityLogColumns(aa ...string) []string {
	var alias string
	if len(aa) > 0 {
		alias = aa[0] + "."
	}

	return []string{
		alias + "id",
		alias + "rel_resource",
		alias + "resource_type",
		alias + "resource_action",
		alias + "ts",
	}
}

// {true true false true true true}

// sortableActivityLogColumns returns all ActivityLog columns flagged as sortable
//
// With optional string arg, all columns are returned aliased
func (Store) sortableActivityLogColumns() map[string]string {
	return map[string]string{
		"id": "id",
	}
}

// internalActivityLogEncoder encodes fields from activitylog.Activity to store.Payload (map)
//
// Encoding is done by using generic approach or by calling encodeActivityLog
// func when rdbms.customEncoder=true
func (s Store) internalActivityLogEncoder(res *activitylog.Activity) store.Payload {
	return store.Payload{
		"id":              res.ID,
		"rel_resource":    res.ResourceID,
		"resource_type":   res.ResourceType,
		"resource_action": res.ResourceAction,
		"ts":              res.Timestamp,
	}
}

// collectActivityLogCursorValues collects values from the given resource that and sets them to the cursor
// to be used for pagination
//
// Values that are collected must come from sortable, unique or primary columns/fields
// At least one of the collected columns must be flagged as unique, otherwise fn appends primary keys at the end
//
// Known issue:
//   when collecting cursor values for query that sorts by unique column with partial index (ie: unique handle on
//   undeleted items)
func (s Store) collectActivityLogCursorValues(res *activitylog.Activity, cc ...*filter.SortExpr) *filter.PagingCursor {
	var (
		cursor = &filter.PagingCursor{LThen: filter.SortExprSet(cc).Reversed()}

		hasUnique bool

		// All known primary key columns

		pkId bool

		collect = func(cc ...*filter.SortExpr) {
			for _, c := range cc {
				switch c.Column {
				case "id":
					cursor.Set(c.Column, res.ID, c.Descending)

					pkId = true

				}
			}
		}
	)

	collect(cc...)
	if !hasUnique || !(pkId && true) {
		collect(&filter.SortExpr{Column: "id", Descending: false})
	}

	return cursor
}

// checkActivityLogConstraints performs lookups (on valid) resource to check if any of the values on unique fields
// already exists in the store
//
// Using built-in constraint checking would be more performant but unfortunately we cannot rely
// on the full support (MySQL does not support conditional indexes)
func (s *Store) checkActivityLogConstraints(ctx context.Context, res *activitylog.Activity) error {
	// Consider resource valid when all fields in unique constraint check lookups
	// have valid (non-empty) value
	//
	// Only string and uint64 are supported for now
	// feel free to add additional types if needed
	var valid = true

	if !valid {
		return nil
	}

	var checks = make([]func() error, 0)

	for _, check := range checks {
		if err := check(); err != nil {
			return err
		}
	}

	return nil
}
