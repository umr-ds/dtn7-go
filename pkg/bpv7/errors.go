// SPDX-FileCopyrightText: 2026 Markus Sommer
//
// SPDX-License-Identifier: GPL-3.0-or-later

package bpv7

import "fmt"

type MalformedAdminRecordError struct {
	message string
	cause   error
}

func NewMalformedAdminRecordError(msg string, cause error) error {
	return MalformedAdminRecordError{msg, cause}
}

func (e MalformedAdminRecordError) Error() string {
	if e.cause == nil {
		return fmt.Sprintf("administrative record was malformed: %v", e.message)
	}
	return fmt.Sprintf("administrative record was malformed: %v: %v", e.message, e.cause)
}

func (e MalformedAdminRecordError) Unwrap() error {
	return e.cause
}

type UnknownAdminRecordTypeError AdminRecordType

func NewUnknownAdminRecordTypeError(adminRecordType AdminRecordType) error {
	return UnknownAdminRecordTypeError(adminRecordType)
}

func (e UnknownAdminRecordTypeError) RecordType() AdminRecordType {
	return AdminRecordType(e)
}

func (e UnknownAdminRecordTypeError) Error() string {
	return fmt.Sprintf("administrative record type %d is unknown", e.RecordType())
}
