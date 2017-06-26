package storage

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"io"

	"github.com/go-openapi/runtime"

	strfmt "github.com/go-openapi/strfmt"
)

// New creates a new storage API client.
func New(transport runtime.ClientTransport, formats strfmt.Registry) *Client {
	return &Client{transport: transport, formats: formats}
}

/*
Client for storage API
*/
type Client struct {
	transport runtime.ClientTransport
	formats   strfmt.Registry
}

/*
CommitImage Commit image
*/
func (a *Client) CommitImage(params *CommitImageParams) (*CommitImageCreated, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewCommitImageParams()
	}

	result, err := a.transport.Submit(&runtime.ClientOperation{
		ID:                 "CommitImage",
		Method:             "PUT",
		PathPattern:        "/snapshot/{store_name}",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json", "application/octet-stream"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &CommitImageReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	})
	if err != nil {
		return nil, err
	}
	return result.(*CommitImageCreated), nil

}

/*
CreateImageStore creates an image store

Creates a location to store images
*/
func (a *Client) CreateImageStore(params *CreateImageStoreParams) (*CreateImageStoreCreated, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewCreateImageStoreParams()
	}

	result, err := a.transport.Submit(&runtime.ClientOperation{
		ID:                 "CreateImageStore",
		Method:             "POST",
		PathPattern:        "/storage",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &CreateImageStoreReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	})
	if err != nil {
		return nil, err
	}
	return result.(*CreateImageStoreCreated), nil

}

/*
CreateVolume creates a volume with metadata that is provided from the personality

Create a volume
*/
func (a *Client) CreateVolume(params *CreateVolumeParams) (*CreateVolumeCreated, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewCreateVolumeParams()
	}

	result, err := a.transport.Submit(&runtime.ClientOperation{
		ID:                 "CreateVolume",
		Method:             "POST",
		PathPattern:        "/storage/volumes",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json", "application/octet-stream"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &CreateVolumeReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	})
	if err != nil {
		return nil, err
	}
	return result.(*CreateVolumeCreated), nil

}

/*
DeleteImage deletes an image

Delete an image by id in an image store
*/
func (a *Client) DeleteImage(params *DeleteImageParams) (*DeleteImageOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewDeleteImageParams()
	}

	result, err := a.transport.Submit(&runtime.ClientOperation{
		ID:                 "DeleteImage",
		Method:             "DELETE",
		PathPattern:        "/storage/{store_name}/info/{id}",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &DeleteImageReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	})
	if err != nil {
		return nil, err
	}
	return result.(*DeleteImageOK), nil

}

/*
GetImage inspects an image

Inspect an image by id in an image store
*/
func (a *Client) GetImage(params *GetImageParams) (*GetImageOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewGetImageParams()
	}

	result, err := a.transport.Submit(&runtime.ClientOperation{
		ID:                 "GetImage",
		Method:             "GET",
		PathPattern:        "/storage/{store_name}/info/{id}",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &GetImageReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	})
	if err != nil {
		return nil, err
	}
	return result.(*GetImageOK), nil

}

/*
GetImageTar gets an image as a tar file

Get an image by id in an image store as a tar file
*/
func (a *Client) GetImageTar(params *GetImageTarParams, writer io.Writer) (*GetImageTarOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewGetImageTarParams()
	}

	result, err := a.transport.Submit(&runtime.ClientOperation{
		ID:                 "GetImageTar",
		Method:             "GET",
		PathPattern:        "/storage/{store_name}/tar/{id}",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &GetImageTarReader{formats: a.formats, writer: writer},
		Context:            params.Context,
		Client:             params.HTTPClient,
	})
	if err != nil {
		return nil, err
	}
	return result.(*GetImageTarOK), nil

}

/*
GetVolume Get info about a volume
*/
func (a *Client) GetVolume(params *GetVolumeParams) (*GetVolumeOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewGetVolumeParams()
	}

	result, err := a.transport.Submit(&runtime.ClientOperation{
		ID:                 "GetVolume",
		Method:             "GET",
		PathPattern:        "/storage/volumes/{name}",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json", "application/octet-stream"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &GetVolumeReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	})
	if err != nil {
		return nil, err
	}
	return result.(*GetVolumeOK), nil

}

/*
ListAllImages List all available images
*/
func (a *Client) ListAllImages(params *ListAllImagesParams) (*ListAllImagesOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewListAllImagesParams()
	}

	result, err := a.transport.Submit(&runtime.ClientOperation{
		ID:                 "ListAllImages",
		Method:             "GET",
		PathPattern:        "/snapshot/{store_name}",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &ListAllImagesReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	})
	if err != nil {
		return nil, err
	}
	return result.(*ListAllImagesOK), nil

}

/*
ListImages retrieves a list of images in an image store

Retrieves a list of images given a list of image IDs, or all images in the image store if no param is passed.
*/
func (a *Client) ListImages(params *ListImagesParams) (*ListImagesOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewListImagesParams()
	}

	result, err := a.transport.Submit(&runtime.ClientOperation{
		ID:                 "ListImages",
		Method:             "GET",
		PathPattern:        "/storage/{store_name}",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &ListImagesReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	})
	if err != nil {
		return nil, err
	}
	return result.(*ListImagesOK), nil

}

/*
ListVolumes Get a list of available volumes
*/
func (a *Client) ListVolumes(params *ListVolumesParams) (*ListVolumesOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewListVolumesParams()
	}

	result, err := a.transport.Submit(&runtime.ClientOperation{
		ID:                 "ListVolumes",
		Method:             "GET",
		PathPattern:        "/storage/volumes",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json", "application/octet-stream"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &ListVolumesReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	})
	if err != nil {
		return nil, err
	}
	return result.(*ListVolumesOK), nil

}

/*
RemoveImage Delete image layer
*/
func (a *Client) RemoveImage(params *RemoveImageParams) (*RemoveImageCreated, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewRemoveImageParams()
	}

	result, err := a.transport.Submit(&runtime.ClientOperation{
		ID:                 "RemoveImage",
		Method:             "DELETE",
		PathPattern:        "/snapshot/{store_name}",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &RemoveImageReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	})
	if err != nil {
		return nil, err
	}
	return result.(*RemoveImageCreated), nil

}

/*
RemoveVolume removes a volume
*/
func (a *Client) RemoveVolume(params *RemoveVolumeParams) (*RemoveVolumeOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewRemoveVolumeParams()
	}

	result, err := a.transport.Submit(&runtime.ClientOperation{
		ID:                 "RemoveVolume",
		Method:             "DELETE",
		PathPattern:        "/storage/volumes/{name}",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &RemoveVolumeReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	})
	if err != nil {
		return nil, err
	}
	return result.(*RemoveVolumeOK), nil

}

/*
UnpackImage creates a new image layer not available for use

Creates a new image layer in an image store that is not available for use
*/
func (a *Client) UnpackImage(params *UnpackImageParams) (*UnpackImageCreated, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewUnpackImageParams()
	}

	result, err := a.transport.Submit(&runtime.ClientOperation{
		ID:                 "UnpackImage",
		Method:             "POST",
		PathPattern:        "/snapshot/{store_name}",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/octet-stream"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &UnpackImageReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	})
	if err != nil {
		return nil, err
	}
	return result.(*UnpackImageCreated), nil

}

/*
VolumeJoin Attach a volume to a container
*/
func (a *Client) VolumeJoin(params *VolumeJoinParams) (*VolumeJoinOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewVolumeJoinParams()
	}

	result, err := a.transport.Submit(&runtime.ClientOperation{
		ID:                 "VolumeJoin",
		Method:             "POST",
		PathPattern:        "/storage/volumes/{name}",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json", "application/octet-stream"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &VolumeJoinReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	})
	if err != nil {
		return nil, err
	}
	return result.(*VolumeJoinOK), nil

}

/*
VolumeStoresList Get a list of available volume store locations
*/
func (a *Client) VolumeStoresList(params *VolumeStoresListParams) (*VolumeStoresListOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewVolumeStoresListParams()
	}

	result, err := a.transport.Submit(&runtime.ClientOperation{
		ID:                 "VolumeStoresList",
		Method:             "GET",
		PathPattern:        "/storage/volumestores",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json", "application/octet-stream"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &VolumeStoresListReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	})
	if err != nil {
		return nil, err
	}
	return result.(*VolumeStoresListOK), nil

}

/*
WriteImage creates a new image layer

Creates a new image layer in an image store
*/
func (a *Client) WriteImage(params *WriteImageParams) (*WriteImageCreated, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewWriteImageParams()
	}

	result, err := a.transport.Submit(&runtime.ClientOperation{
		ID:                 "WriteImage",
		Method:             "POST",
		PathPattern:        "/storage/{store_name}",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/octet-stream"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &WriteImageReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	})
	if err != nil {
		return nil, err
	}
	return result.(*WriteImageCreated), nil

}

// SetTransport changes the transport on the client
func (a *Client) SetTransport(transport runtime.ClientTransport) {
	a.transport = transport
}
