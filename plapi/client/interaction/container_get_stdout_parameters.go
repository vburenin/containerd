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
)

// NewContainerGetStdoutParams creates a new ContainerGetStdoutParams object
// with the default values initialized.
func NewContainerGetStdoutParams() *ContainerGetStdoutParams {
	var ()
	return &ContainerGetStdoutParams{

		timeout: cr.DefaultTimeout,
	}
}

// NewContainerGetStdoutParamsWithTimeout creates a new ContainerGetStdoutParams object
// with the default values initialized, and the ability to set a timeout on a request
func NewContainerGetStdoutParamsWithTimeout(timeout time.Duration) *ContainerGetStdoutParams {
	var ()
	return &ContainerGetStdoutParams{

		timeout: timeout,
	}
}

// NewContainerGetStdoutParamsWithContext creates a new ContainerGetStdoutParams object
// with the default values initialized, and the ability to set a context for a request
func NewContainerGetStdoutParamsWithContext(ctx context.Context) *ContainerGetStdoutParams {
	var ()
	return &ContainerGetStdoutParams{

		Context: ctx,
	}
}

// NewContainerGetStdoutParamsWithHTTPClient creates a new ContainerGetStdoutParams object
// with the default values initialized, and the ability to set a custom HTTPClient for a request
func NewContainerGetStdoutParamsWithHTTPClient(client *http.Client) *ContainerGetStdoutParams {
	var ()
	return &ContainerGetStdoutParams{
		HTTPClient: client,
	}
}

/*ContainerGetStdoutParams contains all the parameters to send to the API endpoint
for the container get stdout operation typically these are written to a http.Request
*/
type ContainerGetStdoutParams struct {

	/*Deadline*/
	Deadline *strfmt.DateTime
	/*ID*/
	ID string

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithTimeout adds the timeout to the container get stdout params
func (o *ContainerGetStdoutParams) WithTimeout(timeout time.Duration) *ContainerGetStdoutParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the container get stdout params
func (o *ContainerGetStdoutParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the container get stdout params
func (o *ContainerGetStdoutParams) WithContext(ctx context.Context) *ContainerGetStdoutParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the container get stdout params
func (o *ContainerGetStdoutParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the container get stdout params
func (o *ContainerGetStdoutParams) WithHTTPClient(client *http.Client) *ContainerGetStdoutParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the container get stdout params
func (o *ContainerGetStdoutParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithDeadline adds the deadline to the container get stdout params
func (o *ContainerGetStdoutParams) WithDeadline(deadline *strfmt.DateTime) *ContainerGetStdoutParams {
	o.SetDeadline(deadline)
	return o
}

// SetDeadline adds the deadline to the container get stdout params
func (o *ContainerGetStdoutParams) SetDeadline(deadline *strfmt.DateTime) {
	o.Deadline = deadline
}

// WithID adds the id to the container get stdout params
func (o *ContainerGetStdoutParams) WithID(id string) *ContainerGetStdoutParams {
	o.SetID(id)
	return o
}

// SetID adds the id to the container get stdout params
func (o *ContainerGetStdoutParams) SetID(id string) {
	o.ID = id
}

// WriteToRequest writes these params to a swagger request
func (o *ContainerGetStdoutParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error

	if o.Deadline != nil {

		// query param deadline
		var qrDeadline strfmt.DateTime
		if o.Deadline != nil {
			qrDeadline = *o.Deadline
		}
		qDeadline := qrDeadline.String()
		if qDeadline != "" {
			if err := r.SetQueryParam("deadline", qDeadline); err != nil {
				return err
			}
		}

	}

	// path param id
	if err := r.SetPathParam("id", o.ID); err != nil {
		return err
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}
