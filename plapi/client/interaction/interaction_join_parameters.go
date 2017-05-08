package interaction

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

// NewInteractionJoinParams creates a new InteractionJoinParams object
// with the default values initialized.
func NewInteractionJoinParams() *InteractionJoinParams {
	var ()
	return &InteractionJoinParams{

		timeout: cr.DefaultTimeout,
	}
}

// NewInteractionJoinParamsWithTimeout creates a new InteractionJoinParams object
// with the default values initialized, and the ability to set a timeout on a request
func NewInteractionJoinParamsWithTimeout(timeout time.Duration) *InteractionJoinParams {
	var ()
	return &InteractionJoinParams{

		timeout: timeout,
	}
}

// NewInteractionJoinParamsWithContext creates a new InteractionJoinParams object
// with the default values initialized, and the ability to set a context for a request
func NewInteractionJoinParamsWithContext(ctx context.Context) *InteractionJoinParams {
	var ()
	return &InteractionJoinParams{

		Context: ctx,
	}
}

// NewInteractionJoinParamsWithHTTPClient creates a new InteractionJoinParams object
// with the default values initialized, and the ability to set a custom HTTPClient for a request
func NewInteractionJoinParamsWithHTTPClient(client *http.Client) *InteractionJoinParams {
	var ()
	return &InteractionJoinParams{
		HTTPClient: client,
	}
}

/*InteractionJoinParams contains all the parameters to send to the API endpoint
for the interaction join operation typically these are written to a http.Request
*/
type InteractionJoinParams struct {

	/*Config*/
	Config *models.InteractionJoinConfig

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithTimeout adds the timeout to the interaction join params
func (o *InteractionJoinParams) WithTimeout(timeout time.Duration) *InteractionJoinParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the interaction join params
func (o *InteractionJoinParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the interaction join params
func (o *InteractionJoinParams) WithContext(ctx context.Context) *InteractionJoinParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the interaction join params
func (o *InteractionJoinParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the interaction join params
func (o *InteractionJoinParams) WithHTTPClient(client *http.Client) *InteractionJoinParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the interaction join params
func (o *InteractionJoinParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithConfig adds the config to the interaction join params
func (o *InteractionJoinParams) WithConfig(config *models.InteractionJoinConfig) *InteractionJoinParams {
	o.SetConfig(config)
	return o
}

// SetConfig adds the config to the interaction join params
func (o *InteractionJoinParams) SetConfig(config *models.InteractionJoinConfig) {
	o.Config = config
}

// WriteToRequest writes these params to a swagger request
func (o *InteractionJoinParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error

	if o.Config == nil {
		o.Config = new(models.InteractionJoinConfig)
	}

	if err := r.SetBodyParam(o.Config); err != nil {
		return err
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}
