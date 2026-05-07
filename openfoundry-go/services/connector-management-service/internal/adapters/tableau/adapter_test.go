package tableau

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/openfoundry/openfoundry-go/services/connector-management-service/internal/adapters"
	"github.com/openfoundry/openfoundry-go/services/connector-management-service/internal/models"
)

func TestValidateConfigAcceptsInlineViewCatalog(t *testing.T) {
	raw := json.RawMessage(`{
		"site_id": "openfoundry",
		"views": [{"view": "Revenue Scorecard", "preview_rows": [{"metric": "revenue", "value": 1024}]}]
	}`)
	require.NoError(t, ValidateConfig(raw))
}

func TestValidateConfigRejectsBareConfig(t *testing.T) {
	require.Error(t, ValidateConfig(json.RawMessage(`{"site_id":"openfoundry"}`)))
}

func TestValidateConfigRequiresSiteIDForResourceTemplate(t *testing.T) {
	raw := json.RawMessage(`{
		"base_url": "https://tableau.example.com/",
		"view_path_template": "/api/3.10/sites/{site_id}/views/{selector}"
	}`)
	err := ValidateConfig(raw)
	require.Error(t, err)
	require.Contains(t, err.Error(), "site_id")
}

func TestDiscoverSourcesReturnsInlineViews(t *testing.T) {
	c := &models.Connection{Config: json.RawMessage(`{
		"site_id": "openfoundry",
		"views": [
			{"view": "Revenue Scorecard"},
			{"view": "Pipeline Health", "display_name": "Pipeline"}
		]
	}`)}
	sources, err := New().DiscoverSources(context.Background(), c, "")
	require.NoError(t, err)
	require.Len(t, sources, 2)
	require.Equal(t, "Revenue Scorecard", sources[0].Selector)
	require.Equal(t, "tableau_view", sources[0].SourceKind)
	require.Equal(t, "Pipeline", sources[1].DisplayName)
}

func TestQueryVirtualTableServesInlineSampleRows(t *testing.T) {
	c := &models.Connection{Config: json.RawMessage(`{
		"site_id": "openfoundry",
		"views": [{
			"view": "Revenue Scorecard",
			"preview_rows": [{"metric": "revenue", "value": 1024}]
		}]
	}`)}
	res, err := New().QueryVirtualTable(context.Background(), c, &adapters.Query{Selector: "Revenue Scorecard"}, "")
	require.NoError(t, err)
	require.Equal(t, 1, res.RowCount)
	require.JSONEq(t, `{"metric":"revenue","value":1024}`, string(res.Rows[0]))
}

func TestStreamArrowReturnsNotImplemented(t *testing.T) {
	_, err := New().StreamArrow(context.Background(), &models.Connection{}, &adapters.Query{}, "")
	require.True(t, errors.Is(err, adapters.ErrNotImplemented))
}

func TestBuildIngestSpecReturnsNotImplemented(t *testing.T) {
	_, err := New().BuildIngestSpec(context.Background(), &models.Connection{}, &adapters.Source{})
	require.True(t, errors.Is(err, adapters.ErrNotImplemented))
}

func TestFactoryProducesFreshAdapter(t *testing.T) {
	f := Factory()
	a := f.New()
	require.NotNil(t, a)
	_, ok := a.(*Adapter)
	require.True(t, ok)
}
