package languages_test

import (
	"astrix/pkg/indexer"
	"context"
	"testing"
	"time"

	_ "astrix/pkg/indexer/languages"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseASTDigest(t *testing.T, cfg indexer.LanguageConfig, code string) *sitter.Node {
	parser := sitter.NewParser()
	parser.SetLanguage(cfg.GetLanguage())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tree, err := parser.ParseCtx(ctx, nil, []byte(code))
	require.NoError(t, err)
	require.NotNil(t, tree)
	return tree.RootNode()
}

func TestExtractDigest_Go(t *testing.T) {
	code := `package service

import (
	"context"
	"fmt"
)

type UserService struct {
	repo UserRepository
}

func NewUserService(r UserRepository) *UserService {
	return &UserService{repo: r}
}

func (s *UserService) GetUser(ctx context.Context, id string) (*User, error) {
	if id == "" {
		return nil, fmt.Errorf("empty id")
	}
	return s.repo.Find(id)
}
`
	cfg, ok := indexer.GetConfigByName("go")
	require.True(t, ok)
	extractor, ok := cfg.(indexer.ASTDigestExtractor)
	require.True(t, ok)

	root := parseASTDigest(t, cfg, code)
	digest := extractor.ExtractDigest("internal/service/user.go", []byte(code), root)

	assert.Contains(t, digest, "// File: internal/service/user.go")
	assert.Contains(t, digest, "package service")
	assert.Contains(t, digest, "type UserService struct")
	assert.Contains(t, digest, "func NewUserService(r UserRepository) *UserService { ... }")
	assert.Contains(t, digest, "func (s *UserService) GetUser(ctx context.Context, id string) (*User, error) { ... }")
	assert.NotContains(t, digest, "empty id") // Corpos omitidos
}

func TestExtractDigest_TypeScript(t *testing.T) {
	code := `import { Injectable } from '@nestjs/common';
import { UserRepository } from './user.repo';

export interface UserResponse {
  id: string;
  name: string;
}

export class UserService {
  constructor(private readonly userRepo: UserRepository) {}

  async findUser(id: string): Promise<UserResponse> {
    const user = await this.userRepo.find(id);
    return { id: user.id, name: user.name };
  }
}
`
	cfg, ok := indexer.GetConfigByName("typescript")
	require.True(t, ok)
	extractor, ok := cfg.(indexer.ASTDigestExtractor)
	require.True(t, ok)

	root := parseASTDigest(t, cfg, code)
	digest := extractor.ExtractDigest("src/user.service.ts", []byte(code), root)

	assert.Contains(t, digest, "// File: src/user.service.ts")
	assert.Contains(t, digest, "export interface UserResponse")
	assert.Contains(t, digest, "export class UserService")
	assert.Contains(t, digest, "findUser(id: string): Promise<UserResponse> { ... }")
	assert.NotContains(t, digest, "const user = await")
}

func TestExtractDigest_Python(t *testing.T) {
	code := `from typing import Optional, List
from pydantic import BaseModel

class UserSchema(BaseModel):
    id: str
    name: str
    email: Optional[str] = None

class UserService:
    def __init__(self, db_client):
        self.db = db_client

    def get_user(self, user_id: str) -> Optional[UserSchema]:
        user_data = self.storage.query(user_id)
        if not user_data:
            return None
        return UserSchema(**user_data)
`
	cfg, ok := indexer.GetConfigByName("python")
	require.True(t, ok)
	extractor, ok := cfg.(indexer.ASTDigestExtractor)
	require.True(t, ok)

	root := parseASTDigest(t, cfg, code)
	digest := extractor.ExtractDigest("app/services/user_service.py", []byte(code), root)

	assert.Contains(t, digest, "# File: app/services/user_service.py")
	assert.Contains(t, digest, "class UserSchema(BaseModel):")
	assert.Contains(t, digest, "class UserService:")
	assert.Contains(t, digest, "def get_user(self, user_id: str) -> Optional[UserSchema]: ...")
	assert.NotContains(t, digest, "user_data = self.storage.query")
}

func TestExtractDigest_Java(t *testing.T) {
	code := `package com.example.service;

import org.springframework.stereotype.Service;

public class OrderService {
    private final OrderRepository orderRepository;

    public OrderService(OrderRepository orderRepository) {
        this.orderRepository = orderRepository;
    }

    public OrderResponse createOrder(CreateOrderRequest request) {
        Order entity = request.toEntity();
        return OrderResponse.from(orderRepository.save(entity));
    }
}
`
	cfg, ok := indexer.GetConfigByName("java")
	require.True(t, ok)
	extractor, ok := cfg.(indexer.ASTDigestExtractor)
	require.True(t, ok)

	root := parseASTDigest(t, cfg, code)
	digest := extractor.ExtractDigest("src/main/java/OrderService.java", []byte(code), root)

	assert.Contains(t, digest, "// File: src/main/java/OrderService.java")
	assert.Contains(t, digest, "package com.example.service;")
	assert.Contains(t, digest, "public class OrderService {")
	assert.Contains(t, digest, "createOrder(CreateOrderRequest request) { ... }")
	assert.NotContains(t, digest, "Order entity = request.toEntity();")
}

func TestExtractDigest_PHP(t *testing.T) {
	code := `<?php

namespace App\Services;

use App\Repositories\UserRepository;

class UserService {
    public function __construct(private UserRepository $userRepo) {}

    public function findById(int $id): ?UserDTO {
        $user = $this->userRepo->find($id);
        return $user ? new UserDTO($user) : null;
    }
}
`
	cfg, ok := indexer.GetConfigByName("php")
	require.True(t, ok)
	extractor, ok := cfg.(indexer.ASTDigestExtractor)
	require.True(t, ok)

	root := parseASTDigest(t, cfg, code)
	digest := extractor.ExtractDigest("app/Services/UserService.php", []byte(code), root)

	assert.Contains(t, digest, "// File: app/Services/UserService.php")
	assert.Contains(t, digest, "namespace App\\Services;")
	assert.Contains(t, digest, "class UserService {")
	assert.Contains(t, digest, "public function findById(int $id): ?UserDTO { ... }")
	assert.NotContains(t, digest, "$user = $this->userRepo->find")
}
