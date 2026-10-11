package downloads

import "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"

// Provider is a ready download client provider plugin.
type Provider struct {
	ID     string
	Client pluginv1connect.DownloadClientProviderServiceClient
}
