package logging

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

// NewLoggingBindParams creates a new LoggingBindParams object
// with the default values initialized.
func NewLoggingBindParams() *LoggingBindParams {
	var ()
	return &LoggingBindParams{

		timeout: cr.DefaultTimeout,
	}
}

// NewLoggingBindParamsWithTimeout creates a new LoggingBindParams object
// with the default values initialized, and the ability to set a timeout on a request
func NewLoggingBindParamsWithTimeout(timeout time.Duration) *LoggingBindParams {
	var ()
	return &LoggingBindParams{

		timeout: timeout,
	}
}

// NewLoggingBindParamsWithContext creates a new LoggingBindParams object
// with the default values initialized, and the ability to set a context for a request
func NewLoggingBindParamsWithContext(ctx context.Context) *LoggingBindParams {
	var ()
	return &LoggingBindParams{

		Context: ctx,
	}
}

// NewLoggingBindParamsWithHTTPClient creates a new LoggingBindParams object
// with the default values initialized, and the ability to set a custom HTTPClient for a request
func NewLoggingBindParamsWithHTTPClient(client *http.Client) *LoggingBindParams {
	var ()
	return &LoggingBindParams{
		HTTPClient: client,
	}
}

/*LoggingBindParams contains all the parameters to send to the API endpoint
for the logging bind operation typically these are written to a http.Request
*/
type LoggingBindParams struct {

	/*Config*/
	Config *models.LoggingBindConfig

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithTimeout adds the timeout to the logging bind params
func (o *LoggingBindParams) WithTimeout(timeout time.Duration) *LoggingBindParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the logging bind params
func (o *LoggingBindParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the logging bind params
func (o *LoggingBindParams) WithContext(ctx context.Context) *LoggingBindParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the logging bind params
func (o *LoggingBindParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the logging bind params
func (o *LoggingBindParams) WithHTTPClient(client *http.Client) *LoggingBindParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the logging bind params
func (o *LoggingBindParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithConfig adds the config to the logging bind params
func (o *LoggingBindParams) WithConfig(config *models.LoggingBindConfig) *LoggingBindParams {
	o.SetConfig(config)
	return o
}

// SetConfig adds the config to the logging bind params
func (o *LoggingBindParams) SetConfig(config *models.LoggingBindConfig) {
	o.Config = config
}

// WriteToRequest writes these params to a swagger request
func (o *LoggingBindParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error

	if o.Config == nil {
		o.Config = new(models.LoggingBindConfig)
	}

	if err := r.SetBodyParam(o.Config); err != nil {
		return err
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}
