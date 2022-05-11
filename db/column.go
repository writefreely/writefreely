/*
 * Copyright © 2019-2022 A Bunch Tell LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package db

import (
	"fmt"
	"strings"
)

type Column struct {
	Name       string
	Type       ColumnType
	Nullable   bool
	PrimaryKey bool
}

func NullableColumn(name string, ty ColumnType) *Column {
	return &Column{
		Name:       name,
		Type:       ty,
		Nullable:   true,
		PrimaryKey: false,
	}
}

func NonNullableColumn(name string, ty ColumnType) *Column {
	return &Column{
		Name:       name,
		Type:       ty,
		Nullable:   false,
		PrimaryKey: false,
	}
}

func PrimaryKeyColumn(name string, ty ColumnType) *Column {
	return &Column{
		Name:       name,
		Type:       ty,
		Nullable:   false,
		PrimaryKey: true,
	}
}

type ColumnType interface {
	Name(DialectType) (string, error)
	Default(DialectType) (string, error)
}

type ColumnTypeInt struct {
	IsSigned   bool
	MaxBytes   int
	MaxDigits  int
	HasDefault bool
	DefaultVal int
}

type ColumnTypeString struct {
	IsFixedLength bool
	MaxChars      int
	HasDefault    bool
	DefaultVal    string
}

type ColumnDefault int

type ColumnTypeBool struct {
	DefaultVal ColumnDefault
}

const (
	NoDefault    ColumnDefault = iota
	DefaultFalse ColumnDefault = iota
	DefaultTrue  ColumnDefault = iota
	DefaultNow   ColumnDefault = iota
)

type ColumnTypeDateTime struct {
	DefaultVal ColumnDefault
}

func (intCol ColumnTypeInt) Name(d DialectType) (string, error) {
	switch d {
	case DialectSQLite:
		return "INTEGER", nil

	case DialectMySQL:
		var colName string
		switch intCol.MaxBytes {
		case 1:
			colName = "TINYINT"
		case 2:
			colName = "SMALLINT"
		case 3:
			colName = "MEDIUMINT"
		case 4:
			colName = "INT"
		default:
			colName = "BIGINT"
		}
		if intCol.MaxDigits > 0 {
			colName = fmt.Sprintf("%s(%d)", colName, intCol.MaxDigits)
		}
		if !intCol.IsSigned {
			colName += " UNSIGNED"
		}
		return colName, nil

	default:
		return "", fmt.Errorf("dialect %d does not support integer columns", d)
	}
}

func (intCol ColumnTypeInt) Default(d DialectType) (string, error) {
	switch d {
	case DialectSQLite, DialectMySQL:
		if intCol.HasDefault {
			return fmt.Sprintf("%d", intCol.DefaultVal), nil
		}
		return "", nil
	default:
		return "", fmt.Errorf("dialect %d does not support defaulted integer columns", d)
	}
}

func (strCol ColumnTypeString) Name(d DialectType) (string, error) {
	switch d {
	case DialectSQLite:
		return "TEXT", nil

	case DialectMySQL:
		if strCol.IsFixedLength {
			if strCol.MaxChars > 0 {
				return fmt.Sprintf("CHAR(%d)", strCol.MaxChars), nil
			}
			return "CHAR", nil
		}

		if strCol.MaxChars <= 0 {
			return "TEXT", nil
		}
		if strCol.MaxChars < (1 << 16) {
			return fmt.Sprintf("VARCHAR(%d)", strCol.MaxChars), nil
		}
		return "TEXT", nil

	default:
		return "", fmt.Errorf("dialect %d does not support string columns", d)
	}
}

func (strCol ColumnTypeString) Default(d DialectType) (string, error) {
	switch d {
	case DialectSQLite, DialectMySQL:
		if strCol.HasDefault {
			return EscapeSimple.SQLEscape(d, strCol.DefaultVal)
		}
		return "", nil
	default:
		return "", fmt.Errorf("dialect %d does not support defaulted string columns", d)
	}
}

func (boolCol ColumnTypeBool) Name(d DialectType) (string, error) {
	switch d {
	case DialectSQLite:
		return "INTEGER", nil
	case DialectMySQL:
		return "BOOL", nil
	default:
		return "", fmt.Errorf("boolean column type not supported for dialect %d", d)
	}
}

func (boolCol ColumnTypeBool) Default(d DialectType) (string, error) {
	switch d {
	case DialectSQLite, DialectMySQL:
		switch boolCol.DefaultVal {
		case NoDefault:
			return "", nil
		case DefaultFalse:
			return "0", nil
		case DefaultTrue:
			return "1", nil
		default:
			return "", fmt.Errorf("boolean columns cannot default to %d for dialect %d", boolCol.DefaultVal, d)
		}
	default:
		return "", fmt.Errorf("dialect %d does not support defaulted boolean columns", d)
	}
}

func (dateTimeCol ColumnTypeDateTime) Name(d DialectType) (string, error) {
	switch d {
	case DialectSQLite, DialectMySQL:
		return "DATETIME", nil
	default:
		return "", fmt.Errorf("datetime column type not supported for dialect %d", d)
	}
}

func (dateTimeCol ColumnTypeDateTime) Default(d DialectType) (string, error) {
	switch d {
	case DialectSQLite, DialectMySQL:
		switch dateTimeCol.DefaultVal {
		case NoDefault:
			return "", nil
		case DefaultNow:
			switch d {
			case DialectSQLite:
				return "CURRENT_TIMESTAMP", nil
			case DialectMySQL:
				return "NOW()", nil
			}
		}
		return "", fmt.Errorf("datetime columns cannot default to %d for dialect %d", dateTimeCol.DefaultVal, d)
	default:
		return "", fmt.Errorf("dialect %d does not support defaulted datetime columns", d)
	}
}

func (c *Column) SetName(name string) *Column {
	c.Name = name
	return c
}

func (c *Column) SetNullable(nullable bool) *Column {
	c.Nullable = nullable
	return c
}

func (c *Column) SetPrimaryKey(pk bool) *Column {
	c.PrimaryKey = pk
	return c
}

func (c *Column) SetType(t ColumnType) *Column {
	c.Type = t
	return c
}

func (c *Column) ToSQL(d DialectType) (string, error) {
	var str strings.Builder

	str.WriteString(c.Name)

	str.WriteString(" ")
	typeStr, err := c.Type.Name(d)
	if err != nil {
		return "", err
	}

	str.WriteString(typeStr)

	if !c.Nullable {
		str.WriteString(" NOT NULL")
	}

	defaultStr, err := c.Type.Default(d)
	if err != nil {
		return "", err
	}
	if len(defaultStr) > 0 {
		str.WriteString(" DEFAULT ")
		str.WriteString(defaultStr)
	}

	if c.PrimaryKey {
		str.WriteString(" PRIMARY KEY")
	}

	return str.String(), nil
}
