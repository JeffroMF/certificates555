package api

import (
	"errors"
	"net/http"

	"go.step.sm/linkedca"

	"github.com/smallstep/certificates/acme"
	"github.com/smallstep/certificates/api/read"
	"github.com/smallstep/certificates/api/render"
	"github.com/smallstep/certificates/authority"
	"github.com/smallstep/certificates/authority/admin"
	"github.com/smallstep/certificates/authority/policy"
)

type policyAdminResponderInterface interface {
	GetAuthorityPolicy(w http.ResponseWriter, r *http.Request)
	CreateAuthorityPolicy(w http.ResponseWriter, r *http.Request)
	UpdateAuthorityPolicy(w http.ResponseWriter, r *http.Request)
	DeleteAuthorityPolicy(w http.ResponseWriter, r *http.Request)
	GetProvisionerPolicy(w http.ResponseWriter, r *http.Request)
	CreateProvisionerPolicy(w http.ResponseWriter, r *http.Request)
	UpdateProvisionerPolicy(w http.ResponseWriter, r *http.Request)
	DeleteProvisionerPolicy(w http.ResponseWriter, r *http.Request)
	GetACMEAccountPolicy(w http.ResponseWriter, r *http.Request)
	CreateACMEAccountPolicy(w http.ResponseWriter, r *http.Request)
	UpdateACMEAccountPolicy(w http.ResponseWriter, r *http.Request)
	DeleteACMEAccountPolicy(w http.ResponseWriter, r *http.Request)
}

// PolicyAdminResponder is responsible for writing ACME admin responses
type PolicyAdminResponder struct {
	auth       adminAuthority
	adminDB    admin.DB
	acmeDB     acme.DB
	isLinkedCA bool
}

// NewACMEAdminResponder returns a new ACMEAdminResponder
func NewPolicyAdminResponder(auth adminAuthority, adminDB admin.DB, acmeDB acme.DB) *PolicyAdminResponder {

	var isLinkedCA bool
	if a, ok := adminDB.(interface{ IsLinkedCA() bool }); ok {
		isLinkedCA = a.IsLinkedCA()
	}

	return &PolicyAdminResponder{
		auth:       auth,
		adminDB:    adminDB,
		acmeDB:     acmeDB,
		isLinkedCA: isLinkedCA,
	}
}

// GetAuthorityPolicy handles the GET /admin/authority/policy request
func (par *PolicyAdminResponder) GetAuthorityPolicy(w http.ResponseWriter, r *http.Request) {

	if err := par.blockLinkedCA(); err != nil {
		render.Error(w, err)
		return
	}

	authorityPolicy, err := par.auth.GetAuthorityPolicy(r.Context())
	if ae, ok := err.(*admin.Error); ok && !ae.IsType(admin.ErrorNotFoundType) {
		render.Error(w, admin.WrapErrorISE(ae, "error retrieving authority policy"))
		return
	}

	if authorityPolicy == nil {
		render.Error(w, admin.NewError(admin.ErrorNotFoundType, "authority policy does not exist"))
		return
	}

	render.ProtoJSONStatus(w, authorityPolicy, http.StatusOK)
}

// CreateAuthorityPolicy handles the POST /admin/authority/policy request
func (par *PolicyAdminResponder) CreateAuthorityPolicy(w http.ResponseWriter, r *http.Request) {

	if err := par.blockLinkedCA(); err != nil {
		render.Error(w, err)
		return
	}

	ctx := r.Context()
	authorityPolicy, err := par.auth.GetAuthorityPolicy(ctx)

	if ae, ok := err.(*admin.Error); ok && !ae.IsType(admin.ErrorNotFoundType) {
		render.Error(w, admin.WrapErrorISE(err, "error retrieving authority policy"))
		return
	}

	if authorityPolicy != nil {
		adminErr := admin.NewError(admin.ErrorConflictType, "authority already has a policy")
		render.Error(w, adminErr)
		return
	}

	var newPolicy = new(linkedca.Policy)
	if err := read.ProtoJSON(r.Body, newPolicy); err != nil {
		render.Error(w, err)
		return
	}

	newPolicy.Deduplicate()

	if err := validatePolicy(newPolicy); err != nil {
		render.Error(w, admin.WrapError(admin.ErrorBadRequestType, err, "error validating authority policy"))
		return
	}

	adm := linkedca.MustAdminFromContext(ctx)

	var createdPolicy *linkedca.Policy
	if createdPolicy, err = par.auth.CreateAuthorityPolicy(ctx, adm, newPolicy); err != nil {
		if isBadRequest(err) {
			render.Error(w, admin.WrapError(admin.ErrorBadRequestType, err, "error storing authority policy"))
			return
		}

		render.Error(w, admin.WrapErrorISE(err, "error storing authority policy"))
		return
	}

	render.ProtoJSONStatus(w, createdPolicy, http.StatusCreated)
}

// UpdateAuthorityPolicy handles the PUT /admin/authority/policy request
func (par *PolicyAdminResponder) UpdateAuthorityPolicy(w http.ResponseWriter, r *http.Request) {

	if err := par.blockLinkedCA(); err != nil {
		render.Error(w, err)
		return
	}

	ctx := r.Context()
	authorityPolicy, err := par.auth.GetAuthorityPolicy(ctx)

	if ae, ok := err.(*admin.Error); ok && !ae.IsType(admin.ErrorNotFoundType) {
		render.Error(w, admin.WrapErrorISE(err, "error retrieving authority policy"))
		return
	}

	if authorityPolicy == nil {
		render.Error(w, admin.NewError(admin.ErrorNotFoundType, "authority policy does not exist"))
		return
	}

	var newPolicy = new(linkedca.Policy)
	if err := read.ProtoJSON(r.Body, newPolicy); err != nil {
		render.Error(w, err)
		return
	}

	newPolicy.Deduplicate()

	if err := validatePolicy(newPolicy); err != nil {
		render.Error(w, admin.WrapError(admin.ErrorBadRequestType, err, "error validating authority policy"))
		return
	}

	adm := linkedca.MustAdminFromContext(ctx)

	var updatedPolicy *linkedca.Policy
	if updatedPolicy, err = par.auth.UpdateAuthorityPolicy(ctx, adm, newPolicy); err != nil {
		if isBadRequest(err) {
			render.Error(w, admin.WrapError(admin.ErrorBadRequestType, err, "error updating authority policy"))
			return
		}

		render.Error(w, admin.WrapErrorISE(err, "error updating authority policy"))
		return
	}

	render.ProtoJSONStatus(w, updatedPolicy, http.StatusOK)
}

// DeleteAuthorityPolicy handles the DELETE /admin/authority/policy request
func (par *PolicyAdminResponder) DeleteAuthorityPolicy(w http.ResponseWriter, r *http.Request) {

	if err := par.blockLinkedCA(); err != nil {
		render.Error(w, err)
		return
	}

	ctx := r.Context()
	authorityPolicy, err := par.auth.GetAuthorityPolicy(ctx)

	if ae, ok := err.(*admin.Error); ok && !ae.IsType(admin.ErrorNotFoundType) {
		render.Error(w, admin.WrapErrorISE(ae, "error retrieving authority policy"))
		return
	}

	if authorityPolicy == nil {
		render.Error(w, admin.NewError(admin.ErrorNotFoundType, "authority policy does not exist"))
		return
	}

	if err := par.auth.RemoveAuthorityPolicy(ctx); err != nil {
		render.Error(w, admin.WrapErrorISE(err, "error deleting authority policy"))
		return
	}

	render.JSONStatus(w, DeleteResponse{Status: "ok"}, http.StatusOK)
}

// GetProvisionerPolicy handles the GET /admin/provisioners/{name}/policy request
func (par *PolicyAdminResponder) GetProvisionerPolicy(w http.ResponseWriter, r *http.Request) {

	if err := par.blockLinkedCA(); err != nil {
		render.Error(w, err)
		return
	}

	prov := linkedca.MustProvisionerFromContext(r.Context())

	provisionerPolicy := prov.GetPolicy()
	if provisionerPolicy == nil {
		render.Error(w, admin.NewError(admin.ErrorNotFoundType, "provisioner policy does not exist"))
		return
	}

	render.ProtoJSONStatus(w, provisionerPolicy, http.StatusOK)
}

// CreateProvisionerPolicy handles the POST /admin/provisioners/{name}/policy request
func (par *PolicyAdminResponder) CreateProvisionerPolicy(w http.ResponseWriter, r *http.Request) {

	if err := par.blockLinkedCA(); err != nil {
		render.Error(w, err)
		return
	}

	ctx := r.Context()
	prov := linkedca.MustProvisionerFromContext(ctx)

	provisionerPolicy := prov.GetPolicy()
	if provisionerPolicy != nil {
		adminErr := admin.NewError(admin.ErrorConflictType, "provisioner %s already has a policy", prov.Name)
		render.Error(w, adminErr)
		return
	}

	var newPolicy = new(linkedca.Policy)
	if err := read.ProtoJSON(r.Body, newPolicy); err != nil {
		render.Error(w, err)
		return
	}

	newPolicy.Deduplicate()

	if err := validatePolicy(newPolicy); err != nil {
		render.Error(w, admin.WrapError(admin.ErrorBadRequestType, err, "error validating provisioner policy"))
		return
	}

	prov.Policy = newPolicy

	if err := par.auth.UpdateProvisioner(ctx, prov); err != nil {
		if isBadRequest(err) {
			render.Error(w, admin.WrapError(admin.ErrorBadRequestType, err, "error creating provisioner policy"))
			return
		}

		render.Error(w, admin.WrapErrorISE(err, "error creating provisioner policy"))
		return
	}

	render.ProtoJSONStatus(w, newPolicy, http.StatusCreated)
}

// UpdateProvisionerPolicy handles the PUT /admin/provisioners/{name}/policy request
func (par *PolicyAdminResponder) UpdateProvisionerPolicy(w http.ResponseWriter, r *http.Request) {

	if err := par.blockLinkedCA(); err != nil {
		render.Error(w, err)
		return
	}

	ctx := r.Context()
	prov := linkedca.MustProvisionerFromContext(ctx)

	provisionerPolicy := prov.GetPolicy()
	if provisionerPolicy == nil {
		render.Error(w, admin.NewError(admin.ErrorNotFoundType, "provisioner policy does not exist"))
		return
	}

	var newPolicy = new(linkedca.Policy)
	if err := read.ProtoJSON(r.Body, newPolicy); err != nil {
		render.Error(w, err)
		return
	}

	newPolicy.Deduplicate()

	if err := validatePolicy(newPolicy); err != nil {
		render.Error(w, admin.WrapError(admin.ErrorBadRequestType, err, "error validating provisioner policy"))
		return
	}

	prov.Policy = newPolicy
	if err := par.auth.UpdateProvisioner(ctx, prov); err != nil {
		if isBadRequest(err) {
			render.Error(w, admin.WrapError(admin.ErrorBadRequestType, err, "error updating provisioner policy"))
			return
		}

		render.Error(w, admin.WrapErrorISE(err, "error updating provisioner policy"))
		return
	}

	render.ProtoJSONStatus(w, newPolicy, http.StatusOK)
}

// DeleteProvisionerPolicy handles the DELETE /admin/provisioners/{name}/policy request
func (par *PolicyAdminResponder) DeleteProvisionerPolicy(w http.ResponseWriter, r *http.Request) {

	if err := par.blockLinkedCA(); err != nil {
		render.Error(w, err)
		return
	}

	ctx := r.Context()
	prov := linkedca.MustProvisionerFromContext(ctx)

	if prov.Policy == nil {
		render.Error(w, admin.NewError(admin.ErrorNotFoundType, "provisioner policy does not exist"))
		return
	}

	// remove the policy
	prov.Policy = nil

	if err := par.auth.UpdateProvisioner(ctx, prov); err != nil {
		render.Error(w, admin.WrapErrorISE(err, "error deleting provisioner policy"))
		return
	}

	render.JSONStatus(w, DeleteResponse{Status: "ok"}, http.StatusOK)
}

func (par *PolicyAdminResponder) GetACMEAccountPolicy(w http.ResponseWriter, r *http.Request) {

	if err := par.blockLinkedCA(); err != nil {
		render.Error(w, err)
		return
	}

	ctx := r.Context()
	eak := linkedca.MustExternalAccountKeyFromContext(ctx)

	eakPolicy := eak.GetPolicy()
	if eakPolicy == nil {
		render.Error(w, admin.NewError(admin.ErrorNotFoundType, "ACME EAK policy does not exist"))
		return
	}

	render.ProtoJSONStatus(w, eakPolicy, http.StatusOK)
}

func (par *PolicyAdminResponder) CreateACMEAccountPolicy(w http.ResponseWriter, r *http.Request) {

	if err := par.blockLinkedCA(); err != nil {
		render.Error(w, err)
		return
	}

	ctx := r.Context()
	prov := linkedca.MustProvisionerFromContext(ctx)
	eak := linkedca.MustExternalAccountKeyFromContext(ctx)

	eakPolicy := eak.GetPolicy()
	if eakPolicy != nil {
		adminErr := admin.NewError(admin.ErrorConflictType, "ACME EAK %s already has a policy", eak.Id)
		render.Error(w, adminErr)
		return
	}

	var newPolicy = new(linkedca.Policy)
	if err := read.ProtoJSON(r.Body, newPolicy); err != nil {
		render.Error(w, err)
		return
	}

	newPolicy.Deduplicate()

	if err := validatePolicy(newPolicy); err != nil {
		render.Error(w, admin.WrapError(admin.ErrorBadRequestType, err, "error validating ACME EAK policy"))
		return
	}

	eak.Policy = newPolicy

	acmeEAK := linkedEAKToCertificates(eak)
	if err := par.acmeDB.UpdateExternalAccountKey(ctx, prov.GetId(), acmeEAK); err != nil {
		render.Error(w, admin.WrapErrorISE(err, "error creating ACME EAK policy"))
		return
	}

	render.ProtoJSONStatus(w, newPolicy, http.StatusCreated)
}

func (par *PolicyAdminResponder) UpdateACMEAccountPolicy(w http.ResponseWriter, r *http.Request) {

	if err := par.blockLinkedCA(); err != nil {
		render.Error(w, err)
		return
	}

	ctx := r.Context()
	prov := linkedca.MustProvisionerFromContext(ctx)
	eak := linkedca.MustExternalAccountKeyFromContext(ctx)

	eakPolicy := eak.GetPolicy()
	if eakPolicy == nil {
		render.Error(w, admin.NewError(admin.ErrorNotFoundType, "ACME EAK policy does not exist"))
		return
	}

	var newPolicy = new(linkedca.Policy)
	if err := read.ProtoJSON(r.Body, newPolicy); err != nil {
		render.Error(w, err)
		return
	}

	newPolicy.Deduplicate()

	if err := validatePolicy(newPolicy); err != nil {
		render.Error(w, admin.WrapError(admin.ErrorBadRequestType, err, "error validating ACME EAK policy"))
		return
	}

	eak.Policy = newPolicy
	acmeEAK := linkedEAKToCertificates(eak)
	if err := par.acmeDB.UpdateExternalAccountKey(ctx, prov.GetId(), acmeEAK); err != nil {
		render.Error(w, admin.WrapErrorISE(err, "error updating ACME EAK policy"))
		return
	}

	render.ProtoJSONStatus(w, newPolicy, http.StatusOK)
}

func (par *PolicyAdminResponder) DeleteACMEAccountPolicy(w http.ResponseWriter, r *http.Request) {

	if err := par.blockLinkedCA(); err != nil {
		render.Error(w, err)
		return
	}

	ctx := r.Context()
	prov := linkedca.MustProvisionerFromContext(ctx)
	eak := linkedca.MustExternalAccountKeyFromContext(ctx)

	eakPolicy := eak.GetPolicy()
	if eakPolicy == nil {
		render.Error(w, admin.NewError(admin.ErrorNotFoundType, "ACME EAK policy does not exist"))
		return
	}

	// remove the policy
	eak.Policy = nil

	acmeEAK := linkedEAKToCertificates(eak)
	if err := par.acmeDB.UpdateExternalAccountKey(ctx, prov.GetId(), acmeEAK); err != nil {
		render.Error(w, admin.WrapErrorISE(err, "error deleting ACME EAK policy"))
		return
	}

	render.JSONStatus(w, DeleteResponse{Status: "ok"}, http.StatusOK)
}

// blockLinkedCA blocks all API operations on linked deployments
func (par *PolicyAdminResponder) blockLinkedCA() error {
	// temporary blocking linked deployments
	if par.isLinkedCA {
		return admin.NewError(admin.ErrorNotImplementedType, "policy operations not yet supported in linked deployments")
	}
	return nil
}

// isBadRequest checks if an error should result in a bad request error
// returned to the client.
func isBadRequest(err error) bool {
	var pe *authority.PolicyError
	isPolicyError := errors.As(err, &pe)
	return isPolicyError && (pe.Typ == authority.AdminLockOut || pe.Typ == authority.EvaluationFailure || pe.Typ == authority.ConfigurationFailure)
}

func validatePolicy(p *linkedca.Policy) error {

	// convert the policy; return early if nil
	options := policy.LinkedToCertificates(p)
	if options == nil {
		return nil
	}

	var err error

	// Initialize a temporary x509 allow/deny policy engine
	if _, err = policy.NewX509PolicyEngine(options.GetX509Options()); err != nil {
		return err
	}

	// Initialize a temporary SSH allow/deny policy engine for host certificates
	if _, err = policy.NewSSHHostPolicyEngine(options.GetSSHOptions()); err != nil {
		return err
	}

	// Initialize a temporary SSH allow/deny policy engine for user certificates
	if _, err = policy.NewSSHUserPolicyEngine(options.GetSSHOptions()); err != nil {
		return err
	}

	return nil
}
