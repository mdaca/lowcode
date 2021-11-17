package documents

import (
	"context"
	"fmt"

	cmpService "github.com/cortezaproject/corteza-server/compose/service"
	cmpTypes "github.com/cortezaproject/corteza-server/compose/types"
	"github.com/cortezaproject/corteza-server/pkg/errors"
	"github.com/cortezaproject/corteza-server/pkg/filter"
	"github.com/cortezaproject/corteza-server/pkg/rbac"
	sysService "github.com/cortezaproject/corteza-server/system/service"
	sysTypes "github.com/cortezaproject/corteza-server/system/types"
)

type (
	composeResources struct {
		settings *sysTypes.AppSettings

		rbac interface {
			SignificantRoles(res rbac.Resource, op string) (aRR, dRR []uint64)
		}

		ac interface {
			CanReadModule(ctx context.Context, r *cmpTypes.Module) bool
			CanReadNamespace(ctx context.Context, r *cmpTypes.Namespace) bool
			CanReadRecord(ctx context.Context, r *cmpTypes.Record) bool
			CanReadRecordValue(ctx context.Context, r *cmpTypes.ModuleField) bool
			CanReadChart(ctx context.Context, r *cmpTypes.Chart) bool
			CanReadPage(ctx context.Context, r *cmpTypes.Page) bool
		}

		ns interface {
			FindByID(context.Context, uint64) (*cmpTypes.Namespace, error)
			Find(context.Context, cmpTypes.NamespaceFilter) (cmpTypes.NamespaceSet, cmpTypes.NamespaceFilter, error)
		}

		mod interface {
			FindByID(context.Context, uint64, uint64) (*cmpTypes.Module, error)
			Find(ctx context.Context, filter cmpTypes.ModuleFilter) (set cmpTypes.ModuleSet, f cmpTypes.ModuleFilter, err error)
		}

		rec interface {
			Find(ctx context.Context, filter cmpTypes.RecordFilter) (set cmpTypes.RecordSet, f cmpTypes.RecordFilter, err error)
		}
	}
)

func ComposeResources() *composeResources {
	return &composeResources{
		settings: sysService.CurrentSettings,
		rbac:     rbac.Global(),
		ac:       cmpService.DefaultAccessControl,
		ns:       cmpService.DefaultNamespace,
		mod:      cmpService.DefaultModule,
		rec:      cmpService.DefaultRecord,
	}
}

func (d composeResources) Namespaces(ctx context.Context, limit uint, cur string) (rsp *Response, err error) {
	return rsp, func() (err error) {
		if !d.settings.Discovery.ComposeNamespaces.Enabled {
			return errors.Internal("compose namespace indexing disabled")
		}

		var (
			nss cmpTypes.NamespaceSet
			f   = cmpTypes.NamespaceFilter{
				Deleted: filter.StateInclusive,
			}
		)

		if f.Paging, err = filter.NewPaging(limit, cur); err != nil {
			return err
		}

		if nss, f, err = d.ns.Find(ctx, f); err != nil {
			return err
		}

		rsp = &Response{
			Documents: make([]Document, len(nss)),
			Filter: Filter{
				Limit:    limit,
				NextPage: f.NextPage,
			},
		}

		for i, ns := range nss {
			rsp.Documents[i].ID = ns.ID
			// where should this link to?
			// namespace root page on the compose?
			//rsp.Documents[i].URL = "@todo"
			doc := &docComposeNamespace{
				ResourceType: "compose:namespace",
				NamespaceID:  ns.ID,
				Name:         ns.Name,
				Handle:       ns.Slug,
				Meta: docPartialComposeNamespaceMeta{
					Subtitle:    ns.Meta.Subtitle,
					Description: ns.Meta.Description,
				},
				Created: makePartialChange(&ns.CreatedAt),
				Updated: makePartialChange(ns.UpdatedAt),
				Deleted: makePartialChange(ns.DeletedAt),
			}

			doc.Security.AllowedRoles, doc.Security.DeniedRoles = d.rbac.SignificantRoles(ns, "read")

			rsp.Documents[i].Source = doc
		}

		return nil
	}()
}

func (d composeResources) Modules(ctx context.Context, namespaceID uint64, limit uint, cur string) (rsp *Response, err error) {
	return rsp, func() (err error) {
		if !d.settings.Discovery.ComposeModules.Enabled {
			return errors.Internal("compose module indexing disabled")
		}

		var (
			ns *cmpTypes.Namespace
			mm cmpTypes.ModuleSet
			f  = cmpTypes.ModuleFilter{
				NamespaceID: namespaceID,
				Deleted:     filter.StateInclusive,
			}
		)

		if f.Paging, err = filter.NewPaging(limit, cur); err != nil {
			return
		}

		if mm, f, err = d.mod.Find(ctx, f); err != nil {
			return
		}

		rsp = &Response{
			Documents: make([]Document, len(mm)),
			Filter: Filter{
				Limit:    limit,
				NextPage: f.NextPage,
			},
		}

		if ns, err = d.ns.FindByID(ctx, namespaceID); err != nil {
			return
		}

		nsPartial := docPartialComposeNamespace{
			NamespaceID: namespaceID,
			Name:        ns.Name,
			Handle:      ns.Slug,
		}

		for i, mod := range mm {
			rsp.Documents[i].ID = mod.ID
			// Where should this link to?
			// module edit screen in the administration? does this make sense?
			//rsp.Documents[i].URL = "@todo"
			doc := &docComposeModule{
				ResourceType: "compose:module",
				ModuleID:     mod.ID,
				Name:         mod.Name,
				Handle:       mod.Handle,
				Namespace:    nsPartial,
				Fields: func() []*docPartialComposeModuleField {
					out := make([]*docPartialComposeModuleField, len(mod.Fields))
					for i, f := range mod.Fields {
						out[i] = &docPartialComposeModuleField{
							Name:  f.Name,
							Label: f.Label,
						}
					}
					return out
				}(),
				Created: makePartialChange(&mod.CreatedAt),
				Updated: makePartialChange(mod.UpdatedAt),
				Deleted: makePartialChange(mod.DeletedAt),
			}

			doc.Security.AllowedRoles, doc.Security.DeniedRoles = d.rbac.SignificantRoles(mod, "read")

			rsp.Documents[i].Source = doc
		}

		return nil
	}()
}

func (d composeResources) Records(ctx context.Context, namespaceID, moduleID uint64, limit uint, cur string) (rsp *Response, err error) {
	return rsp, func() (err error) {
		if !d.settings.Discovery.ComposeRecords.Enabled {
			return errors.Internal("compose record indexing disabled")
		}

		var (
			ns  *cmpTypes.Namespace
			mod *cmpTypes.Module
			rr  cmpTypes.RecordSet
			f   = cmpTypes.RecordFilter{
				NamespaceID: namespaceID,
				ModuleID:    moduleID,
				Deleted:     filter.StateInclusive,
			}
		)

		if f.Paging, err = filter.NewPaging(limit, cur); err != nil {
			return err
		}

		if rr, f, err = d.rec.Find(ctx, f); err != nil {
			return err
		}

		rsp = &Response{
			Documents: make([]Document, len(rr)),
			Filter: Filter{
				Limit:    limit,
				NextPage: f.NextPage,
			},
		}

		// @todo handle unreadable (access-control) namespaces
		if ns, err = d.ns.FindByID(ctx, namespaceID); err != nil {
			return
		}

		nsPartial := docPartialComposeNamespace{
			NamespaceID: namespaceID,
			Name:        ns.Name,
			Handle:      ns.Slug,
		}

		// @todo handle unreadable (access-control) modules
		if mod, err = d.mod.FindByID(ctx, namespaceID, moduleID); err != nil {
			return
		}

		modPartial := docPartialComposeModule{
			ModuleID: f.ModuleID,
			Name:     mod.Name,
			Handle:   mod.Handle,
		}

		for i, rec := range rr {
			rsp.Documents[i].ID = rec.ID
			// where should this link to? record page in the compose?
			//rsp.Documents[i].URL = "@todo"
			doc := &docComposeRecord{
				ResourceType: "compose:record",
				RecordID:     rec.ID,
				Namespace:    nsPartial,
				Module:       modPartial,
				Values:       d.recordValues(ctx, rec),
				Created:      makePartialChange(&rec.CreatedAt),
				Updated:      makePartialChange(rec.UpdatedAt),
				Deleted:      makePartialChange(rec.DeletedAt),
			}

			doc.Security.AllowedRoles, doc.Security.DeniedRoles = d.rbac.SignificantRoles(rec, "read")

			rsp.Documents[i].Source = doc
		}

		return nil
	}()
}

func (d composeResources) recordValues(ctx context.Context, rec *cmpTypes.Record) map[string][]interface{} {
	var (
		rval = make(map[string][]interface{})
	)

	if rec.GetModule() == nil {
		return nil
	}

	_ = rec.GetModule().Fields.Walk(func(f *cmpTypes.ModuleField) error {
		if !d.ac.CanReadRecordValue(ctx, f) {
			return nil
		}

		var (
			rv = rec.Values.FilterByName(f.Name)
			vv = make([]interface{}, 0, len(rv))
		)

		if len(rv) == 0 {
			return nil
		}

		for _, val := range rv {
			// refs needs to be casted to string (json & unsigned 64-bit integers)!
			if f.IsRef() {
				vv = append(vv, fmt.Sprintf("%d", val.Ref))
				continue
			}

			if tmp, err := val.Cast(f); err == nil {
				vv = append(vv, tmp)
			}

		}

		if len(vv) == 0 {
			return nil
		}

		rval[f.Name] = vv

		return nil
	})

	return rval
}

func (d composeResources) UpdateNamespacesMeta(ctx context.Context) (rsp interface{}, err error) {
	return
}

func (d composeResources) UpdateModuleMeta(ctx context.Context) (rsp interface{}, err error) {
	return
}
