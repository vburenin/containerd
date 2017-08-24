package storage

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"net/http"
	"time"

	"golang.org/x/net/context"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	cr "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/swag"

	strfmt "github.com/go-openapi/strfmt"
)

// NewExportArchiveParams creates a new ExportArchiveParams object
// with the default values initialized.
func NewExportArchiveParams() *ExportArchiveParams {
	var ()
	return &ExportArchiveParams{

		timeout: cr.DefaultTimeout,
	}
}

// NewExportArchiveParamsWithTimeout creates a new ExportArchiveParams object
// with the default values initialized, and the ability to set a timeout on a request
func NewExportArchiveParamsWithTimeout(timeout time.Duration) *ExportArchiveParams {
	var ()
	return &ExportArchiveParams{

		timeout: timeout,
	}
}

// NewExportArchiveParamsWithContext creates a new ExportArchiveParams object
// with the default values initialized, and the ability to set a context for a request
func NewExportArchiveParamsWithContext(ctx context.Context) *ExportArchiveParams {
	var ()
	return &ExportArchiveParams{

		Context: ctx,
	}
}

// NewExportArchiveParamsWithHTTPClient creates a new ExportArchiveParams object
// with the default values initialized, and the ability to set a custom HTTPClient for a request
func NewExportArchiveParamsWithHTTPClient(client *http.Client) *ExportArchiveParams {
	var ()
	return &ExportArchiveParams{
		HTTPClient: client,
	}
}

/*ExportArchiveParams contains all the parameters to send to the API endpoint
for the export archive operation typically these are written to a http.Request
*/
type ExportArchiveParams struct {

	/*Ancestor*/
	Ancestor *string
	/*AncestorStore*/
	AncestorStore *string
	/*Data*/
	Data bool
	/*DeviceID*/
	DeviceID string
	/*FilterSpec*/
	FilterSpec *string
	/*Store*/
	Store string

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithTimeout adds the timeout to the export archive params
func (o *ExportArchiveParams) WithTimeout(timeout time.Duration) *ExportArchiveParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the export archive params
func (o *ExportArchiveParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the export archive params
func (o *ExportArchiveParams) WithContext(ctx context.Context) *ExportArchiveParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the export archive params
func (o *ExportArchiveParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the export archive params
func (o *ExportArchiveParams) WithHTTPClient(client *http.Client) *ExportArchiveParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the export archive params
func (o *ExportArchiveParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithAncestor adds the ancestor to the export archive params
func (o *ExportArchiveParams) WithAncestor(ancestor *string) *ExportArchiveParams {
	o.SetAncestor(ancestor)
	return o
}

// SetAncestor adds the ancestor to the export archive params
func (o *ExportArchiveParams) SetAncestor(ancestor *string) {
	o.Ancestor = ancestor
}

// WithAncestorStore adds the ancestorStore to the export archive params
func (o *ExportArchiveParams) WithAncestorStore(ancestorStore *string) *ExportArchiveParams {
	o.SetAncestorStore(ancestorStore)
	return o
}

// SetAncestorStore adds the ancestorStore to the export archive params
func (o *ExportArchiveParams) SetAncestorStore(ancestorStore *string) {
	o.AncestorStore = ancestorStore
}

// WithData adds the data to the export archive params
func (o *ExportArchiveParams) WithData(data bool) *ExportArchiveParams {
	o.SetData(data)
	return o
}

// SetData adds the data to the export archive params
func (o *ExportArchiveParams) SetData(data bool) {
	o.Data = data
}

// WithDeviceID adds the deviceID to the export archive params
func (o *ExportArchiveParams) WithDeviceID(deviceID string) *ExportArchiveParams {
	o.SetDeviceID(deviceID)
	return o
}

// SetDeviceID adds the deviceId to the export archive params
func (o *ExportArchiveParams) SetDeviceID(deviceID string) {
	o.DeviceID = deviceID
}

// WithFilterSpec adds the filterSpec to the export archive params
func (o *ExportArchiveParams) WithFilterSpec(filterSpec *string) *ExportArchiveParams {
	o.SetFilterSpec(filterSpec)
	return o
}

// SetFilterSpec adds the filterSpec to the export archive params
func (o *ExportArchiveParams) SetFilterSpec(filterSpec *string) {
	o.FilterSpec = filterSpec
}

// WithStore adds the store to the export archive params
func (o *ExportArchiveParams) WithStore(store string) *ExportArchiveParams {
	o.SetStore(store)
	return o
}

// SetStore adds the store to the export archive params
func (o *ExportArchiveParams) SetStore(store string) {
	o.Store = store
}

// WriteToRequest writes these params to a swagger request
func (o *ExportArchiveParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error

	if o.Ancestor != nil {

		// query param ancestor
		var qrAncestor string
		if o.Ancestor != nil {
			qrAncestor = *o.Ancestor
		}
		qAncestor := qrAncestor
		if qAncestor != "" {
			if err := r.SetQueryParam("ancestor", qAncestor); err != nil {
				return err
			}
		}

	}

	if o.AncestorStore != nil {

		// query param ancestorStore
		var qrAncestorStore string
		if o.AncestorStore != nil {
			qrAncestorStore = *o.AncestorStore
		}
		qAncestorStore := qrAncestorStore
		if qAncestorStore != "" {
			if err := r.SetQueryParam("ancestorStore", qAncestorStore); err != nil {
				return err
			}
		}

	}

	// query param data
	qrData := o.Data
	qData := swag.FormatBool(qrData)
	if qData != "" {
		if err := r.SetQueryParam("data", qData); err != nil {
			return err
		}
	}

	// query param deviceID
	qrDeviceID := o.DeviceID
	qDeviceID := qrDeviceID
	if qDeviceID != "" {
		if err := r.SetQueryParam("deviceID", qDeviceID); err != nil {
			return err
		}
	}

	if o.FilterSpec != nil {

		// query param filterSpec
		var qrFilterSpec string
		if o.FilterSpec != nil {
			qrFilterSpec = *o.FilterSpec
		}
		qFilterSpec := qrFilterSpec
		if qFilterSpec != "" {
			if err := r.SetQueryParam("filterSpec", qFilterSpec); err != nil {
				return err
			}
		}

	}

	// query param store
	qrStore := o.Store
	qStore := qrStore
	if qStore != "" {
		if err := r.SetQueryParam("store", qStore); err != nil {
			return err
		}
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}
