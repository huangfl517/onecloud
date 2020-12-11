package options

import (
	"fmt"

	"yunion.io/x/jsonutils"
)

type SkuSyncOptions struct {
	// 云平台名称
	// example: Google
	Provider string `json:"provider,omitempty" help:"cloud provider name"`

	// 区域ID
	CloudregionIds []string `json:"cloudregion_ids" help:"cloud region id list"`
}

func (opts *SkuSyncOptions) Params() (jsonutils.JSONObject, error) {
	return jsonutils.Marshal(opts), nil
}

type SkuTaskQueryOptions struct {
	// 异步任务ID
	TaskIds []string `json:"task_ids" help:"task ids"`
}

func (opts *SkuTaskQueryOptions) Params() (jsonutils.JSONObject, error) {
	if len(opts.TaskIds) == 0 {
		return nil, fmt.Errorf("task_ids is empty")
	}

	params := jsonutils.NewDict()
	params.Set("task_ids", jsonutils.Marshal(opts.TaskIds))
	return params, nil
}
