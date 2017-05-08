package kv

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

// NewDeleteValueParams creates a new DeleteValueParams object
// with the default values initialized.
func NewDeleteValueParams() *DeleteValueParams {
	var ()
	return &DeleteValueParams{

		timeout: cr.DefaultTimeout,
	}
}

// NewDeleteValueParamsWithTimeout creates a new DeleteValueParams object
// with the default values initialized, and the ability to set a timeout on a request
func NewDeleteValueParamsWithTimeout(timeout time.Duration) *DeleteValueParams {
	var ()
	return &DeleteValueParams{

		timeout: timeout,
	}
}

// NewDeleteValueParamsWithContext creates a new DeleteValueParams object
// with the default values initialized, and the ability to set a context for a request
func NewDeleteValueParamsWithContext(ctx context.Context) *DeleteValueParams {
	var ()
	return &DeleteValueParams{

		Context: ctx,
	}
}

// NewDeleteValueParamsWithHTTPClient creates a new DeleteValueParams object
// with the default values initialized, and the ability to set a custom HTTPClient for a request
func NewDeleteValueParamsWithHTTPClient(client *http.Client) *DeleteValueParams {
	var ()
	return &DeleteValueParams{
		HTTPClient: client,
	}
}

/*DeleteValueParams contains all the parameters to send to the API endpoint
for the delete value operation typically these are written to a http.Request
*/
type DeleteValueParams struct {

	/*Key*/
	Key string

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithTimeout adds the timeout to the delete value params
func (o *DeleteValueParams) WithTimeout(timeout time.Duration) *DeleteValueParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the delete value params
func (o *DeleteValueParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the delete value params
func (o *DeleteValueParams) WithContext(ctx context.Context) *DeleteValueParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the delete value params
func (o *DeleteValueParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the delete value params
func (o *DeleteValueParams) WithHTTPClient(client *http.Client) *DeleteValueParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the delete value params
func (o *DeleteValueParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithKey adds the key to the delete value params
func (o *DeleteValueParams) WithKey(key string) *DeleteValueParams {
	o.SetKey(key)
	return o
}

// SetKey adds the key to the delete value params
func (o *DeleteValueParams) SetKey(key string) {
	o.Key = key
}

// WriteToRequest writes these params to a swagger request
func (o *DeleteValueParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error

	// path param key
	if err := r.SetPathParam("key", o.Key); err != nil {
		return err
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}
