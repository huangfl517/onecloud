package azure

import (
	"context"
	"strings"

	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/onecloud/pkg/cloudprovider"
	"yunion.io/x/pkg/utils"

	"github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2017-10-01/storage"
)

type SZone struct {
	region *SRegion

	iwires    []cloudprovider.ICloudWire
	istorages []cloudprovider.ICloudStorage

	storageTypes []string
	Name         string
	host         *SHost
}

func (self *SZone) GetMetadata() *jsonutils.JSONDict {
	return nil
}

func (self *SZone) GetId() string {
	return self.region.client.providerId
}

func (self *SZone) GetName() string {
	return self.region.GetName()
}

func (self *SZone) GetGlobalId() string {
	return self.region.GetGlobalId()
}

func (self *SZone) IsEmulated() bool {
	return true
}

func (self *SZone) GetStatus() string {
	return "enable"
}

func (self *SZone) Refresh() error {
	// do nothing
	return nil
}

func (self *SZone) getHost() *SHost {
	if self.host == nil {
		self.host = &SHost{zone: self}
	}
	return self.host
}

func (self *SZone) getStorageTypes() error {
	storageClinet := storage.NewSkusClientWithBaseURI(self.region.client.baseUrl, self.region.SubscriptionID)
	storageClinet.Authorizer = self.region.client.authorizer

	if skuList, err := storageClinet.List(context.Background()); err != nil {
		return err
	} else {
		for _, sku := range *skuList.Value {
			if len(*sku.Locations) > 0 && (*sku.Locations)[0] == self.region.Name {
				if !utils.IsInStringArray(string(sku.Name), self.storageTypes) {
					self.storageTypes = append(self.storageTypes, string(sku.Name))
				}
			}
		}
	}
	return nil
}

func (self *SZone) GetIRegion() cloudprovider.ICloudRegion {
	return self.region
}

func (self *SZone) fetchStorages() error {
	if len(self.storageTypes) == 0 {
		if err := self.getStorageTypes(); err != nil {
			return err
		}
	}
	self.istorages = make([]cloudprovider.ICloudStorage, len(self.storageTypes))
	for i, storageType := range self.storageTypes {
		storage := SStorage{zone: self, storageType: storageType}
		self.istorages[i] = &storage
	}
	return nil
}

func (self *SZone) GetIStorages() ([]cloudprovider.ICloudStorage, error) {
	if err := self.fetchStorages(); err != nil {
		return nil, err
	}
	return self.istorages, nil
}

func (self *SZone) GetIStorageById(id string) (cloudprovider.ICloudStorage, error) {
	if self.istorages == nil {
		self.fetchStorages()
	}
	for i := 0; i < len(self.istorages); i += 1 {
		if self.istorages[i].GetGlobalId() == id {
			return self.istorages[i], nil
		}
	}
	return nil, cloudprovider.ErrNotFound
}

func (self *SZone) getStorageByType(storageType string) (*SStorage, error) {
	if storages, err := self.GetIStorages(); err != nil {
		return nil, err
	} else {
		for i := 0; i < len(storages); i += 1 {
			_storage := storages[i].(*SStorage)
			if strings.ToLower(_storage.storageType) == strings.ToLower(storageType) {
				return _storage, nil
			}
		}
	}
	return nil, cloudprovider.ErrNotFound
}

func (self *SZone) GetIHostById(id string) (cloudprovider.ICloudHost, error) {
	host := self.getHost()
	if host.GetGlobalId() == id {
		return host, nil
	}
	return nil, cloudprovider.ErrNotFound
}

func (self *SZone) GetIHosts() ([]cloudprovider.ICloudHost, error) {
	return []cloudprovider.ICloudHost{self.getHost()}, nil
}

func (self *SZone) addWire(wire *SWire) {
	if self.iwires == nil {
		self.iwires = make([]cloudprovider.ICloudWire, 0)
	}
	self.iwires = append(self.iwires, wire)
}

func (self *SZone) GetIWires() ([]cloudprovider.ICloudWire, error) {
	return self.iwires, nil
}

func (self *SZone) getNetworkById(networId string) *SNetwork {
	log.Debugf("Search in wires %d", len(self.iwires))
	for i := 0; i < len(self.iwires); i += 1 {
		log.Debugf("Search in wire %s", self.iwires[i].GetName())
		wire := self.iwires[i].(*SWire)
		net := wire.getNetworkById(networId)
		if net != nil {
			return net
		}
	}
	return nil
}
