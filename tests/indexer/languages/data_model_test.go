package languages_test

import (
	"astrix/pkg/indexer"
	"testing"

	_ "astrix/pkg/indexer/languages"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractDataModels_TypeScript(t *testing.T) {
	tsCode := `
export interface CreateUserDto {
  id: string;
  name: string;
  email: string;
  age?: number;
  roles?: string[];
}

export type Point = {
  x: number;
  y: number;
  label?: string;
};

export class UpdateUserDto {
  @IsString()
  @IsNotEmpty()
  name: string;

  @IsOptional()
  @IsInt()
  age?: number;

  someMethod() {
    return true;
  }
}
`
	cfg, ok := indexer.GetConfigByName("typescript")
	require.True(t, ok)
	extractor, ok := cfg.(indexer.DataModelExtractor)
	require.True(t, ok)

	rootNode := parseAST(t, cfg, tsCode)
	models := extractor.ExtractDataModels("proj-1", "src/dto/user.ts", []byte(tsCode), rootNode)
	require.Len(t, models, 3)

	// 1. Interface CreateUserDto
	m1 := models[0]
	assert.Equal(t, "CreateUserDto", m1.Name)
	assert.Equal(t, "interface", m1.Kind)
	require.Len(t, m1.Fields, 5)
	assert.Equal(t, "id", m1.Fields[0].Name)
	assert.Equal(t, "string", m1.Fields[0].Type)
	assert.True(t, m1.Fields[0].Required)
	assert.Equal(t, "age", m1.Fields[3].Name)
	assert.False(t, m1.Fields[3].Required)

	// 2. Type Alias Point
	m2 := models[1]
	assert.Equal(t, "Point", m2.Name)
	assert.Equal(t, "type", m2.Kind)
	require.Len(t, m2.Fields, 3)
	assert.Equal(t, "x", m2.Fields[0].Name)
	assert.Equal(t, "number", m2.Fields[0].Type)
	assert.True(t, m2.Fields[0].Required)
	assert.Equal(t, "label", m2.Fields[2].Name)
	assert.False(t, m2.Fields[2].Required)

	// 3. Class UpdateUserDto (methods stripped)
	m3 := models[2]
	assert.Equal(t, "UpdateUserDto", m3.Name)
	assert.Equal(t, "class", m3.Kind)
	require.Len(t, m3.Fields, 2)
	assert.Equal(t, "name", m3.Fields[0].Name)
	assert.Equal(t, "string", m3.Fields[0].Type)
	assert.Equal(t, "age", m3.Fields[1].Name)
	assert.Equal(t, "number", m3.Fields[1].Type)
}

func TestExtractDataModels_Python(t *testing.T) {
	pyCode := `
from pydantic import BaseModel, Field
from typing import Optional, List
from dataclasses import dataclass

class UserSchema(BaseModel):
    id: int
    username: str
    email: str
    bio: Optional[str] = None
    tags: List[str] = []

    def full_name(self):
        return self.username

@dataclass
class OrderItem:
    item_id: str
    quantity: int = 1
    price: float = 0.0
`
	cfg, ok := indexer.GetConfigByName("python")
	require.True(t, ok)
	extractor, ok := cfg.(indexer.DataModelExtractor)
	require.True(t, ok)

	rootNode := parseAST(t, cfg, pyCode)
	models := extractor.ExtractDataModels("proj-1", "app/schemas.py", []byte(pyCode), rootNode)
	require.Len(t, models, 2)

	// UserSchema
	m1 := models[0]
	assert.Equal(t, "UserSchema", m1.Name)
	assert.Equal(t, "class", m1.Kind)
	require.Len(t, m1.Fields, 5)
	assert.Equal(t, "id", m1.Fields[0].Name)
	assert.Equal(t, "int", m1.Fields[0].Type)
	assert.True(t, m1.Fields[0].Required)
	assert.Equal(t, "bio", m1.Fields[3].Name)
	assert.Equal(t, "Optional[str]", m1.Fields[3].Type)
	assert.False(t, m1.Fields[3].Required)

	// OrderItem
	m2 := models[1]
	assert.Equal(t, "OrderItem", m2.Name)
	assert.Equal(t, "class", m2.Kind)
	require.Len(t, m2.Fields, 3)
	assert.Equal(t, "item_id", m2.Fields[0].Name)
	assert.Equal(t, "str", m2.Fields[0].Type)
	assert.True(t, m2.Fields[0].Required)
	assert.Equal(t, "quantity", m2.Fields[1].Name)
	assert.False(t, m2.Fields[1].Required)
}

func TestExtractDataModels_Go(t *testing.T) {
	goCode := `
package models

type User struct {
	ID        string    ` + "`" + `json:"id" db:"id"` + "`" + `
	Email     string    ` + "`" + `json:"email" db:"email"` + "`" + `
	Nickname  *string   ` + "`" + `json:"nickname,omitempty"` + "`" + `
	IsActive  bool      ` + "`" + `json:"is_active"` + "`" + `
}

type OrderRequest struct {
	UserID  string ` + "`" + `json:"user_id"` + "`" + `
	Amount  float64 ` + "`" + `json:"amount"` + "`" + `
}
`
	cfg, ok := indexer.GetConfigByName("go")
	require.True(t, ok)
	extractor, ok := cfg.(indexer.DataModelExtractor)
	require.True(t, ok)

	rootNode := parseAST(t, cfg, goCode)
	models := extractor.ExtractDataModels("proj-1", "pkg/models/user.go", []byte(goCode), rootNode)
	require.Len(t, models, 2)

	m1 := models[0]
	assert.Equal(t, "User", m1.Name)
	assert.Equal(t, "struct", m1.Kind)
	require.Len(t, m1.Fields, 4)
	assert.Equal(t, "ID", m1.Fields[0].Name)
	assert.Equal(t, "string", m1.Fields[0].Type)
	assert.True(t, m1.Fields[0].Required)
	assert.Equal(t, "Nickname", m1.Fields[2].Name)
	assert.Equal(t, "*string", m1.Fields[2].Type)
	assert.False(t, m1.Fields[2].Required) // has omitempty / pointer
}

func TestExtractDataModels_Java(t *testing.T) {
	javaCode := `
package com.example.dto;

public record UserRecord(Long id, String username, String email) {}

public class UserEntity {
    private Long id;
    private String username;
    private String email;
    private Integer age;

    public Long getId() {
        return id;
    }
}
`
	cfg, ok := indexer.GetConfigByName("java")
	require.True(t, ok)
	extractor, ok := cfg.(indexer.DataModelExtractor)
	require.True(t, ok)

	rootNode := parseAST(t, cfg, javaCode)
	models := extractor.ExtractDataModels("proj-1", "src/main/java/com/example/dto/UserRecord.java", []byte(javaCode), rootNode)
	require.Len(t, models, 2)

	// Record
	m1 := models[0]
	assert.Equal(t, "UserRecord", m1.Name)
	assert.Equal(t, "record", m1.Kind)
	require.Len(t, m1.Fields, 3)
	assert.Equal(t, "id", m1.Fields[0].Name)
	assert.Equal(t, "Long", m1.Fields[0].Type)

	// Class
	m2 := models[1]
	assert.Equal(t, "UserEntity", m2.Name)
	assert.Equal(t, "class", m2.Kind)
	require.Len(t, m2.Fields, 4)
	assert.Equal(t, "id", m2.Fields[0].Name)
	assert.Equal(t, "Long", m2.Fields[0].Type)
}

func TestExtractDataModels_PHP(t *testing.T) {
	phpCode := `<?php
namespace App\DTO;

class CreateUserDTO {
    public string $name;
    public string $email;
    public ?int $age = null;

    public function isValid(): bool {
        return true;
    }
}

readonly class UserRecord {
    public function __construct(
        public string $id,
        public string $email,
        public ?string $bio = null,
    ) {}
}
`
	cfg, ok := indexer.GetConfigByName("php")
	require.True(t, ok)
	extractor, ok := cfg.(indexer.DataModelExtractor)
	require.True(t, ok)

	rootNode := parseAST(t, cfg, phpCode)
	models := extractor.ExtractDataModels("proj-1", "src/DTO/UserDTO.php", []byte(phpCode), rootNode)
	require.Len(t, models, 2)

	m1 := models[0]
	assert.Equal(t, "CreateUserDTO", m1.Name)
	assert.Equal(t, "class", m1.Kind)
	require.Len(t, m1.Fields, 3)
	assert.Equal(t, "name", m1.Fields[0].Name)
	assert.Equal(t, "string", m1.Fields[0].Type)
	assert.True(t, m1.Fields[0].Required)
	assert.Equal(t, "age", m1.Fields[2].Name)
	assert.False(t, m1.Fields[2].Required)

	m2 := models[1]
	assert.Equal(t, "UserRecord", m2.Name)
	assert.Equal(t, "class", m2.Kind)
	require.Len(t, m2.Fields, 3)
	assert.Equal(t, "id", m2.Fields[0].Name)
	assert.Equal(t, "string", m2.Fields[0].Type)
	assert.Equal(t, "bio", m2.Fields[2].Name)
	assert.False(t, m2.Fields[2].Required)
}
