package driver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/rancher/rancher/pkg/kontainer-engine/drivers/options"
	"github.com/rancher/rancher/pkg/kontainer-engine/types"
	"github.com/sirupsen/logrus"
	waldurclient "github.com/waldur/go-client"
)

type Driver struct {
	driverCapabilities types.Capabilities
}

type state struct {
	// The displayed name of the cluster
	DisplayName string
	// The name of this cluster
	Name string
	// The URL of Waldur site
	Url string
	// The token of Waldur user
	Token string
	// The UUID of the offering in Waldur
	OfferingUuid string
	// The UUID of the offering plan in Waldur
	PlanUuid string
	// The UUID of the project in Waldur
	ProjectUuid string
	// Number of CPU cores for the cluster
	Cores int
	// Size of memory for the cluster (GB)
	Ram int
	// Size of disk memory for the cluster (GB)
	Storage int
}

func getStateFromOptions(driverOptions *types.DriverOptions) state {
	s := state{}
	s.DisplayName = options.GetValueFromDriverOptions(driverOptions, types.StringType, "display-name", "displayName").(string)
	s.Name = options.GetValueFromDriverOptions(driverOptions, types.StringType, "name").(string)
	s.Url = options.GetValueFromDriverOptions(driverOptions, types.StringType, "waldur-url", "waldurUrl").(string)
	s.Token = options.GetValueFromDriverOptions(driverOptions, types.StringType, "waldur-token", "waldurToken").(string)
	s.OfferingUuid = options.GetValueFromDriverOptions(driverOptions, types.StringType, "offering-uuid", "offeringUuid").(string)
	s.PlanUuid = options.GetValueFromDriverOptions(driverOptions, types.StringType, "plan-uuid", "planUuid").(string)
	s.ProjectUuid = options.GetValueFromDriverOptions(driverOptions, types.StringType, "project-uuid", "projectUuid").(string)
	s.Cores = options.GetValueFromDriverOptions(driverOptions, types.IntType, "cores").(int)
	s.Ram = options.GetValueFromDriverOptions(driverOptions, types.IntType, "ram").(int)
	s.Storage = options.GetValueFromDriverOptions(driverOptions, types.IntType, "storage").(int)
	return s
}

func storeState(info *types.ClusterInfo, state *state) error {
	bytes, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if info.Metadata == nil {
		info.Metadata = map[string]string{}
	}
	info.Metadata["state"] = string(bytes)
	return nil
}

func getState(info *types.ClusterInfo) (state, error) {
	state := state{}

	err := json.Unmarshal([]byte(info.Metadata["state"]), &state)

	if err != nil {
		logrus.Errorf("Error encountered while unmarshalling state: %v", err)
	}

	return state, err
}

func getWaldurClient(s *state) (*waldurclient.ClientWithResponses, error) {
	hc := http.Client{}
	auth, err := waldurclient.NewTokenAuth(s.Token)
	if err != nil {
		logrus.Errorf("Error while creating token auth %s", err)
		return nil, err
	}

	client, err := waldurclient.NewClientWithResponses(s.Url, waldurclient.WithHTTPClient(&hc), waldurclient.WithRequestEditorFn(auth.Intercept))
	if err != nil {
		logrus.Errorf("Error creating Waldur client %s", err)
		return nil, err
	}

	return client, nil
}

// GetDriverCreateOptions returns cli flags that are used in create
func (d *Driver) GetDriverCreateOptions(ctx context.Context) (*types.DriverFlags, error) {
	driverFlag := types.DriverFlags{
		Options: make(map[string]*types.Flag),
	}
	driverFlag.Options["name"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The internal name of the cluster in Rancher",
	}
	driverFlag.Options["display-name"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The displayed name of the cluster in the Rancher UI.",
	}
	driverFlag.Options["waldur-token"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The token of Waldur user",
	}
	driverFlag.Options["waldur-url"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The URL of Waldur site",
	}
	driverFlag.Options["project-uuid"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The UUID of the project in Waldur",
	}
	driverFlag.Options["offering-uuid"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The UUID of the offering in Waldur",
	}
	driverFlag.Options["plan-uuid"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The UUID of the offering plan in Waldur",
	}
	driverFlag.Options["cores"] = &types.Flag{
		Type:  types.IntType,
		Usage: "The number of CPU cores for the cluster",
	}
	driverFlag.Options["ram"] = &types.Flag{
		Type:  types.IntType,
		Usage: "The size of memory for the cluster (GB)",
	}
	driverFlag.Options["storage"] = &types.Flag{
		Type:  types.IntType,
		Usage: "The size of disk memory for the cluster (GB)",
	}

	return &driverFlag, nil
}

// GetDriverUpdateOptions returns cli flags that are used in update
func (d *Driver) GetDriverUpdateOptions(ctx context.Context) (*types.DriverFlags, error) {
	return &types.DriverFlags{}, nil
}

// Create creates the cluster. clusterInfo is only set when we are retrying a failed or interrupted create
func (d *Driver) Create(ctx context.Context, opts *types.DriverOptions, clusterInfo *types.ClusterInfo) (*types.ClusterInfo, error) {
	s := getStateFromOptions(opts)
	logrus.Infof("Creating cluster %s", s.Name)

	projectUri := fmt.Sprintf("%s/api/projects/%s/", s.Url, s.ProjectUuid)
	offeringUri := fmt.Sprintf("%s/api/marketplace-public-offerings/%s/", s.Url, s.OfferingUuid)
	planUri := fmt.Sprintf("%s/api/marketplace-public-offerings/%s/plans/%s/", s.Url, s.OfferingUuid, s.PlanUuid)

	var attributes interface{} = map[string]interface{}{
		"name": s.Name,
	}

	acceptingTermsOfService := true

	limits := map[string]int{
		"cores":   s.Cores,
		"ram":     s.Ram * 1024,
		"storage": s.Storage * 1024,
	}

	requestType := waldurclient.RequestTypesCreate

	payload := waldurclient.MarketplaceOrdersCreateJSONRequestBody{
		AcceptingTermsOfService: &acceptingTermsOfService,
		Attributes:              &attributes,
		Limits:                  &limits,
		Offering:                offeringUri,
		Project:                 projectUri,
		Type:                    &requestType,
		Plan:                    &planUri,
	}

	client, err := getWaldurClient(&s)
	if err != nil {
		logrus.Errorf("Error while creating token auth %s", err)
		return nil, err
	}

	resp, err := client.MarketplaceOrdersCreateWithResponse(ctx, payload)

	if err != nil {
		logrus.Errorf("Error calling API for tenant creation: %v", err)
		return nil, err
	}

	if resp.StatusCode() != 201 {
		responseBody := string(resp.Body[:])
		logrus.Errorf("Unable to create an instance %s, code %d, details: %s", s.Name, resp.StatusCode(), responseBody)
		msg := fmt.Sprintf("Unable to create an instance %s, code %d", s.Name, resp.StatusCode())
		return nil, errors.New(msg)
	}

	logrus.Infof("Successfully created tenant %s", s.Name)
	info := types.ClusterInfo{}
	err = storeState(&info, &s)
	if err != nil {
		return &info, err
	}

	return &info, nil

}

// Update updates the cluster
func (d *Driver) Update(ctx context.Context, clusterInfo *types.ClusterInfo, opts *types.DriverOptions) (*types.ClusterInfo, error) {
	return nil, nil
}

// PostCheck does post action after provisioning
func (d *Driver) PostCheck(ctx context.Context, clusterInfo *types.ClusterInfo) (*types.ClusterInfo, error) {
	return nil, nil
}

// Remove removes the cluster
func (d *Driver) Remove(ctx context.Context, clusterInfo *types.ClusterInfo) error {
	return nil
}

func (d *Driver) GetVersion(ctx context.Context, clusterInfo *types.ClusterInfo) (*types.KubernetesVersion, error) {
	_, err := getState(clusterInfo)

	if err != nil {
		return nil, err
	}

	// TODO: get cluster info using state

	if err != nil {
		return nil, fmt.Errorf("error getting cluster info: %v", err)
	}

	// return &types.KubernetesVersion{Version: *cluster.KubernetesVersion}, nil
	return nil, nil
}

func (d *Driver) SetVersion(ctx context.Context, clusterInfo *types.ClusterInfo, version *types.KubernetesVersion) error {
	return nil
}
func (d *Driver) GetClusterSize(ctx context.Context, clusterInfo *types.ClusterInfo) (*types.NodeCount, error) {
	return nil, nil
}
func (d *Driver) SetClusterSize(ctx context.Context, clusterInfo *types.ClusterInfo, count *types.NodeCount) error {
	return nil
}

// Get driver capabilities
func (d *Driver) GetCapabilities(ctx context.Context) (*types.Capabilities, error) {
	return &d.driverCapabilities, nil
}

// Remove legacy service account token
func (d *Driver) RemoveLegacyServiceAccount(ctx context.Context, clusterInfo *types.ClusterInfo) error {
	return nil
}

func (d *Driver) ETCDSave(ctx context.Context, clusterInfo *types.ClusterInfo, opts *types.DriverOptions, snapshotName string) error {
	return nil
}
func (d *Driver) ETCDRestore(ctx context.Context, clusterInfo *types.ClusterInfo, opts *types.DriverOptions, snapshotName string) (*types.ClusterInfo, error) {
	return nil, nil
}
func (d *Driver) ETCDRemoveSnapshot(ctx context.Context, clusterInfo *types.ClusterInfo, opts *types.DriverOptions, snapshotName string) error {
	return nil
}

func (d *Driver) GetK8SCapabilities(ctx context.Context, opts *types.DriverOptions) (*types.K8SCapabilities, error) {
	return nil, nil
}
