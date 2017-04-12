package containers

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

// NewCreateParams creates a new CreateParams object
// with the default values initialized.
func NewCreateParams() *CreateParams {
	var ()
	return &CreateParams{

		timeout: cr.DefaultTimeout,
	}
}

// NewCreateParamsWithTimeout creates a new CreateParams object
// with the default values initialized, and the ability to set a timeout on a request
func NewCreateParamsWithTimeout(timeout time.Duration) *CreateParams {
	var ()
	return &CreateParams{

		timeout: timeout,
	}
}

// NewCreateParamsWithContext creates a new CreateParams object
// with the default values initialized, and the ability to set a context for a request
func NewCreateParamsWithContext(ctx context.Context) *CreateParams {
	var ()
	return &CreateParams{

		Context: ctx,
	}
}

// NewCreateParamsWithHTTPClient creates a new CreateParams object
// with the default values initialized, and the ability to set a custom HTTPClient for a request
func NewCreateParamsWithHTTPClient(client *http.Client) *CreateParams {
	var ()
	return &CreateParams{
		HTTPClient: client,
	}
}

/*CreateParams contains all the parameters to send to the API endpoint
for the create operation typically these are written to a http.Request
*/
type CreateParams struct {

	/*CreateConfig*/
	CreateConfig *models.ContainerCreateConfig
	/*Name*/
	Name *string

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithTimeout adds the timeout to the create params
func (o *CreateParams) WithTimeout(timeout time.Duration) *CreateParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the create params
func (o *CreateParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the create params
func (o *CreateParams) WithContext(ctx context.Context) *CreateParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the create params
func (o *CreateParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the create params
func (o *CreateParams) WithHTTPClient(client *http.Client) *CreateParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the create params
func (o *CreateParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithCreateConfig adds the createConfig to the create params
func (o *CreateParams) WithCreateConfig(createConfig *models.ContainerCreateConfig) *CreateParams {
	o.SetCreateConfig(createConfig)
	return o
}

// SetCreateConfig adds the createConfig to the create params
func (o *CreateParams) SetCreateConfig(createConfig *models.ContainerCreateConfig) {
	o.CreateConfig = createConfig
}

// WithName adds the name to the create params
func (o *CreateParams) WithName(name *string) *CreateParams {
	o.SetName(name)
	return o
}

// SetName adds the name to the create params
func (o *CreateParams) SetName(name *string) {
	o.Name = name
}

// WriteToRequest writes these params to a swagger request
func (o *CreateParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	r.SetTimeout(o.timeout)
	var res []error

	if o.CreateConfig == nil {
		o.CreateConfig = new(models.ContainerCreateConfig)
	}

	if err := r.SetBodyParam(o.CreateConfig); err != nil {
		return err
	}

	if o.Name != nil {

		// query param name
		var qrName string
		if o.Name != nil {
			qrName = *o.Name
		}
		qName := qrName
		if qName != "" {
			if err := r.SetQueryParam("name", qName); err != nil {
				return err
			}
		}

	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}
