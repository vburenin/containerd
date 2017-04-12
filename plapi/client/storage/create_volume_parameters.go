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

	strfmt "github.com/go-openapi/strfmt"

	"github.com/containerd/containerd/plapi/models"
)

// NewCreateVolumeParams creates a new CreateVolumeParams object
// with the default values initialized.
func NewCreateVolumeParams() *CreateVolumeParams {
	var ()
	return &CreateVolumeParams{

		timeout: cr.DefaultTimeout,
	}
}

// NewCreateVolumeParamsWithTimeout creates a new CreateVolumeParams object
// with the default values initialized, and the ability to set a timeout on a request
func NewCreateVolumeParamsWithTimeout(timeout time.Duration) *CreateVolumeParams {
	var ()
	return &CreateVolumeParams{

		timeout: timeout,
	}
}

// NewCreateVolumeParamsWithContext creates a new CreateVolumeParams object
// with the default values initialized, and the ability to set a context for a request
func NewCreateVolumeParamsWithContext(ctx context.Context) *CreateVolumeParams {
	var ()
	return &CreateVolumeParams{

		Context: ctx,
	}
}

// NewCreateVolumeParamsWithHTTPClient creates a new CreateVolumeParams object
// with the default values initialized, and the ability to set a custom HTTPClient for a request
func NewCreateVolumeParamsWithHTTPClient(client *http.Client) *CreateVolumeParams {
	var ()
	return &CreateVolumeParams{
		HTTPClient: client,
	}
}

/*CreateVolumeParams contains all the parameters to send to the API endpoint
for the create volume operation typically these are written to a http.Request
*/
type CreateVolumeParams struct {

	/*VolumeRequest*/
	VolumeRequest *models.VolumeRequest

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithTimeout adds the timeout to the create volume params
func (o *CreateVolumeParams) WithTimeout(timeout time.Duration) *CreateVolumeParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the create volume params
func (o *CreateVolumeParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the create volume params
func (o *CreateVolumeParams) WithContext(ctx context.Context) *CreateVolumeParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the create volume params
func (o *CreateVolumeParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the create volume params
func (o *CreateVolumeParams) WithHTTPClient(client *http.Client) *CreateVolumeParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the create volume params
func (o *CreateVolumeParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithVolumeRequest adds the volumeRequest to the create volume params
func (o *CreateVolumeParams) WithVolumeRequest(volumeRequest *models.VolumeRequest) *CreateVolumeParams {
	o.SetVolumeRequest(volumeRequest)
	return o
}

// SetVolumeRequest adds the volumeRequest to the create volume params
func (o *CreateVolumeParams) SetVolumeRequest(volumeRequest *models.VolumeRequest) {
	o.VolumeRequest = volumeRequest
}

// WriteToRequest writes these params to a swagger request
func (o *CreateVolumeParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	r.SetTimeout(o.timeout)
	var res []error

	if o.VolumeRequest == nil {
		o.VolumeRequest = new(models.VolumeRequest)
	}

	if err := r.SetBodyParam(o.VolumeRequest); err != nil {
		return err
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}
