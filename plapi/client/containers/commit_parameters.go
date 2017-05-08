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
	"github.com/go-openapi/swag"

	strfmt "github.com/go-openapi/strfmt"
)

// NewCommitParams creates a new CommitParams object
// with the default values initialized.
func NewCommitParams() *CommitParams {
	var ()
	return &CommitParams{

		timeout: cr.DefaultTimeout,
	}
}

// NewCommitParamsWithTimeout creates a new CommitParams object
// with the default values initialized, and the ability to set a timeout on a request
func NewCommitParamsWithTimeout(timeout time.Duration) *CommitParams {
	var ()
	return &CommitParams{

		timeout: timeout,
	}
}

// NewCommitParamsWithContext creates a new CommitParams object
// with the default values initialized, and the ability to set a context for a request
func NewCommitParamsWithContext(ctx context.Context) *CommitParams {
	var ()
	return &CommitParams{

		Context: ctx,
	}
}

// NewCommitParamsWithHTTPClient creates a new CommitParams object
// with the default values initialized, and the ability to set a custom HTTPClient for a request
func NewCommitParamsWithHTTPClient(client *http.Client) *CommitParams {
	var ()
	return &CommitParams{
		HTTPClient: client,
	}
}

/*CommitParams contains all the parameters to send to the API endpoint
for the commit operation typically these are written to a http.Request
*/
type CommitParams struct {

	/*Handle*/
	Handle string
	/*WaitTime*/
	WaitTime *int32

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithTimeout adds the timeout to the commit params
func (o *CommitParams) WithTimeout(timeout time.Duration) *CommitParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the commit params
func (o *CommitParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the commit params
func (o *CommitParams) WithContext(ctx context.Context) *CommitParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the commit params
func (o *CommitParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the commit params
func (o *CommitParams) WithHTTPClient(client *http.Client) *CommitParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the commit params
func (o *CommitParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithHandle adds the handle to the commit params
func (o *CommitParams) WithHandle(handle string) *CommitParams {
	o.SetHandle(handle)
	return o
}

// SetHandle adds the handle to the commit params
func (o *CommitParams) SetHandle(handle string) {
	o.Handle = handle
}

// WithWaitTime adds the waitTime to the commit params
func (o *CommitParams) WithWaitTime(waitTime *int32) *CommitParams {
	o.SetWaitTime(waitTime)
	return o
}

// SetWaitTime adds the waitTime to the commit params
func (o *CommitParams) SetWaitTime(waitTime *int32) {
	o.WaitTime = waitTime
}

// WriteToRequest writes these params to a swagger request
func (o *CommitParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error

	// path param handle
	if err := r.SetPathParam("handle", o.Handle); err != nil {
		return err
	}

	if o.WaitTime != nil {

		// query param wait_time
		var qrWaitTime int32
		if o.WaitTime != nil {
			qrWaitTime = *o.WaitTime
		}
		qWaitTime := swag.FormatInt32(qrWaitTime)
		if qWaitTime != "" {
			if err := r.SetQueryParam("wait_time", qWaitTime); err != nil {
				return err
			}
		}

	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}
