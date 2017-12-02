package main

import (
	"github.com/go-accounting/coa"
	"github.com/go-accounting/deb"
	financialstatementsspacecoa "github.com/go-accounting/financial-statements-space-coa"
)

var newSpace func(map[string]interface{}, *string, *string) (interface{}, error)
var newKeyValueStore func(map[string]interface{}, *string) (interface{}, error)
var LoadSymbolFunction func(string, string) (interface{}, error)
var spaceSettings map[string]interface{}
var keyValueStoreSettings map[string]interface{}

func NewDataSource(settings map[string]interface{}, user *string, coaid *string) (interface{}, error) {
	if newSpace == nil {
		spaceSettings = map[string]interface{}{}
		for k, v := range settings["Space"].(map[interface{}]interface{}) {
			spaceSettings[k.(string)] = v
		}
		symbol, err := LoadSymbolFunction(spaceSettings["PluginFile"].(string), "NewSpace")
		if err != nil {
			return nil, err
		}
		newSpace = symbol.(func(map[string]interface{}, *string, *string) (interface{}, error))
	}
	if newKeyValueStore == nil {
		keyValueStoreSettings = map[string]interface{}{}
		for k, v := range settings["AccountsRepository"].(map[interface{}]interface{}) {
			keyValueStoreSettings[k.(string)] = v
		}
		symbol, err := LoadSymbolFunction(keyValueStoreSettings["PluginFile"].(string), "NewKeyValueStore")
		if err != nil {
			return nil, err
		}
		newKeyValueStore = symbol.(func(map[string]interface{}, *string) (interface{}, error))
	}
	space, err := newSpace(spaceSettings, user, coaid)
	if err != nil {
		return nil, err
	}
	keyValueStore, err := newKeyValueStore(keyValueStoreSettings, user)
	if err != nil {
		return nil, err
	}
	return financialstatementsspacecoa.NewDataSource(
		space.(deb.Space),
		coa.NewCoaRepository(keyValueStore.(coa.KeyValueStore)),
		coaid,
	)
}
