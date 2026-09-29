package languages_test

import (
	"astrix/pkg/indexer/languages"
	"context"
	"testing"
	"time"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseWithConfig(t *testing.T, config languages.TypeScriptLanguageConfig, content []byte) *sitter.Node {
	parser := sitter.NewParser()
	parser.SetLanguage(config.GetLanguage())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tree, err := parser.ParseCtx(ctx, nil, content)
	require.NoError(t, err)
	return tree.RootNode()
}

func TestExtractDependencies_TypeScript_NestJS(t *testing.T) {
	tsCode := `
import { Injectable, Inject, Controller } from '@nestjs/common';
import { AddressService } from './address.service';
import { BaseController } from '../base/base.controller';

@Controller('address')
export class AddressController extends BaseController {
    constructor(
        private readonly addressService: AddressService,
        @Inject('CUSTOM_LOGGER') private readonly logger: LoggerService,
    ) {
        super();
    }

    async getAddress() {
        const helper = new AddressHelper();
        return this.addressService.findAll();
    }
}
`
	cfg := &languages.TypeScriptLanguageConfig{}
	parser := sitter.NewParser()
	parser.SetLanguage(cfg.GetLanguage())
	tree, err := parser.ParseCtx(context.Background(), nil, []byte(tsCode))
	require.NoError(t, err)

	edges := cfg.ExtractDependencies("proj-test", "src/api/address.controller.ts", []byte(tsCode), tree.RootNode())
	require.NotEmpty(t, edges)

	foundExtends := false
	foundService := false
	foundLogger := false
	foundHelper := false

	for _, e := range edges {
		assert.Equal(t, "AddressController", e.SourceSymbol)
		if e.TargetSymbol == "BaseController" && e.RelationshipType == "uses" {
			foundExtends = true
		}
		if e.TargetSymbol == "AddressService" && e.RelationshipType == "injects" {
			foundService = true
		}
		if e.TargetSymbol == "CUSTOM_LOGGER" && e.RelationshipType == "injects" {
			foundLogger = true
		}
		if e.TargetSymbol == "AddressHelper" && e.RelationshipType == "instantiates" {
			foundHelper = true
		}
	}

	assert.True(t, foundExtends, "Deve extrair extends BaseController")
	assert.True(t, foundService, "Deve extrair injeção de AddressService")
	assert.True(t, foundLogger, "Deve extrair injeção de CUSTOM_LOGGER via @Inject")
	assert.True(t, foundHelper, "Deve extrair instanciação de AddressHelper")
}

func TestExtractDependencies_Java_Spring(t *testing.T) {
	javaCode := `
package com.example.demo;

import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.RestController;

@RestController
public class OrderService extends BaseService {
    @Autowired
    private PaymentClient paymentClient;

    private final OrderRepository orderRepository;

    public OrderService(OrderRepository orderRepository) {
        this.orderRepository = orderRepository;
        NotificationSender sender = new NotificationSender();
    }
}
`
	cfg := &languages.JavaLanguageConfig{}
	parser := sitter.NewParser()
	parser.SetLanguage(cfg.GetLanguage())
	tree, err := parser.ParseCtx(context.Background(), nil, []byte(javaCode))
	require.NoError(t, err)

	edges := cfg.ExtractDependencies("proj-test", "src/main/OrderService.java", []byte(javaCode), tree.RootNode())
	require.NotEmpty(t, edges)

	foundExtends := false
	foundAutowired := false
	foundConstructor := false
	foundInstantiates := false

	for _, e := range edges {
		assert.Equal(t, "OrderService", e.SourceSymbol)
		if e.TargetSymbol == "BaseService" && e.RelationshipType == "uses" {
			foundExtends = true
		}
		if e.TargetSymbol == "PaymentClient" && e.RelationshipType == "injects" {
			foundAutowired = true
		}
		if e.TargetSymbol == "OrderRepository" && e.RelationshipType == "injects" {
			foundConstructor = true
		}
		if e.TargetSymbol == "NotificationSender" && e.RelationshipType == "instantiates" {
			foundInstantiates = true
		}
	}

	assert.True(t, foundExtends)
	assert.True(t, foundAutowired)
	assert.True(t, foundConstructor)
	assert.True(t, foundInstantiates)
}

func TestExtractDependencies_PHP_Laravel(t *testing.T) {
	phpCode := `<?php
namespace App\Http\Controllers;

class UserController extends BaseController {
    public function __construct(
        private UserService $userService,
        protected UserRepository $userRepo
    ) {
        $validator = new UserValidator();
    }
}
`
	cfg := &languages.PhpLanguageConfig{}
	parser := sitter.NewParser()
	parser.SetLanguage(cfg.GetLanguage())
	tree, err := parser.ParseCtx(context.Background(), nil, []byte(phpCode))
	require.NoError(t, err)

	edges := cfg.ExtractDependencies("proj-test", "app/Http/Controllers/UserController.php", []byte(phpCode), tree.RootNode())
	require.NotEmpty(t, edges)

	foundExtends := false
	foundService := false
	foundRepo := false
	foundValidator := false

	for _, e := range edges {
		assert.Equal(t, "UserController", e.SourceSymbol)
		if e.TargetSymbol == "BaseController" && e.RelationshipType == "uses" {
			foundExtends = true
		}
		if e.TargetSymbol == "UserService" && e.RelationshipType == "injects" {
			foundService = true
		}
		if e.TargetSymbol == "UserRepository" && e.RelationshipType == "injects" {
			foundRepo = true
		}
		if e.TargetSymbol == "UserValidator" && e.RelationshipType == "instantiates" {
			foundValidator = true
		}
	}

	assert.True(t, foundExtends)
	assert.True(t, foundService)
	assert.True(t, foundRepo)
	assert.True(t, foundValidator)
}

func TestExtractDependencies_Python_FastAPI(t *testing.T) {
	pyCode := `
from fastapi import Depends

class OrderHandler(BaseHandler):
    def __init__(self, repo: OrderRepository):
        self.repo = repo
        self.helper = PaymentHelper()

def create_order(service: OrderService = Depends(get_order_service)):
    pass
`
	cfg := &languages.PythonLanguageConfig{}
	parser := sitter.NewParser()
	parser.SetLanguage(cfg.GetLanguage())
	tree, err := parser.ParseCtx(context.Background(), nil, []byte(pyCode))
	require.NoError(t, err)

	edges := cfg.ExtractDependencies("proj-test", "app/orders.py", []byte(pyCode), tree.RootNode())
	require.NotEmpty(t, edges)

	foundExtends := false
	foundInitParam := false
	foundHelper := false
	foundFastApi := false

	for _, e := range edges {
		if e.SourceSymbol == "OrderHandler" {
			if e.TargetSymbol == "BaseHandler" && e.RelationshipType == "uses" {
				foundExtends = true
			}
			if e.TargetSymbol == "OrderRepository" && e.RelationshipType == "injects" {
				foundInitParam = true
			}
			if e.TargetSymbol == "PaymentHelper" && e.RelationshipType == "instantiates" {
				foundHelper = true
			}
		}
		if e.SourceSymbol == "create_order" && e.TargetSymbol == "get_order_service" && e.RelationshipType == "injects" {
			foundFastApi = true
		}
	}

	assert.True(t, foundExtends)
	assert.True(t, foundInitParam)
	assert.True(t, foundHelper)
	assert.True(t, foundFastApi)
}

func TestExtractDependencies_Go(t *testing.T) {
	goCode := `
package service

type UserService struct {
	repo UserRepository
	cfg  *AppConfig
}

func NewUserService(repo UserRepository, logger Logger) *UserService {
	return &UserService{repo: repo}
}
`
	cfg := &languages.GoLanguageConfig{}
	parser := sitter.NewParser()
	parser.SetLanguage(cfg.GetLanguage())
	tree, err := parser.ParseCtx(context.Background(), nil, []byte(goCode))
	require.NoError(t, err)

	edges := cfg.ExtractDependencies("proj-test", "internal/service/user.go", []byte(goCode), tree.RootNode())
	require.NotEmpty(t, edges)

	foundStructRepo := false
	foundStructCfg := false
	foundFactoryLogger := false

	for _, e := range edges {
		if e.SourceSymbol == "UserService" {
			if e.TargetSymbol == "UserRepository" && e.RelationshipType == "injects" {
				foundStructRepo = true
			}
			if e.TargetSymbol == "AppConfig" && e.RelationshipType == "injects" {
				foundStructCfg = true
			}
			if e.TargetSymbol == "Logger" && e.RelationshipType == "injects" {
				foundFactoryLogger = true
			}
		}
	}

	assert.True(t, foundStructRepo)
	assert.True(t, foundStructCfg)
	assert.True(t, foundFactoryLogger)
}
