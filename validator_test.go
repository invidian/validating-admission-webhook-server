package main

import (
	"encoding/json"
	"testing"
)

func TestAddTypeNoJsonPath(t *testing.T) {
	rule := ConfigRule{
		Name: "TestAddTypeNoJsonPath",
	}
	validator := NewValidator()
	if err := validator.AddRule("Foo", rule); err == nil {
		t.Errorf("Validator should reject rules without JSONPath defined")
	}
}

func TestAddRuleNoKind(t *testing.T) {
	rule := ConfigRule{
		Name:     "TestAddRuleNoKind",
		Jsonpath: "{}",
	}
	validator := NewValidator()
	if err := validator.AddRule("", rule); err == nil {
		t.Errorf("Validator should reject rules without Type defined")
	}
}

func TestAddRuleNoName(t *testing.T) {
	rule := ConfigRule{
		Jsonpath: "{}",
	}
	validator := NewValidator()
	if err := validator.AddRule("Foo", rule); err == nil {
		t.Errorf("Validator should reject rules without Name defined")
	}
}

func TestValidateEmpty(t *testing.T) {
	validator := NewValidator()
	if err := validator.Validate("Empty", "Foo", "{}"); err != nil {
		t.Errorf("Empty validator should never return error: %s", err)
	}
}

func TestAddRuleNoRegex(t *testing.T) {
	rule := ConfigRule{
		Name:     "TestAddRuleNoRegex",
		Jsonpath: "{}",
	}
	validator := NewValidator()
	if err := validator.AddRule("Foo", rule); err != nil {
		t.Errorf("Validator should accept rules without Regexp defined: %s", err)
	}
}

func TestAddRuleMalformedJsonpath(t *testing.T) {
	rule := ConfigRule{
		Name:     "TestAddRuleMalformedJsonpath",
		Jsonpath: "{",
	}
	validator := NewValidator()
	if err := validator.AddRule("Foo", rule); err == nil {
		t.Errorf("Malformed JSONPath shouldn't create validator rule")
	}
}

func TestAddRuleMalformedRegexp(t *testing.T) {
	rule := ConfigRule{
		Name:     "TestAddRuleMalformedRegexp",
		Jsonpath: "{}",
		Regexp:   "[",
	}
	validator := NewValidator()
	if err := validator.AddRule("Foo", rule); err == nil {
		t.Errorf("Malformed regexp shouldn't create validator rule")
	}
}

func TestAddRule(t *testing.T) {
	rule := ConfigRule{
		Name:     "TestAddRule",
		Jsonpath: "{}",
		Regexp:   ".*",
	}
	validator := NewValidator()
	if err := validator.AddRule("Foo", rule); err != nil {
		t.Errorf("Validator shouldn't fail adding rule: %s", err)
	}
	if i := len(validator.rules["Foo"]); i != 1 {
		t.Errorf("Validator should accept valid rules. Created rules: %d", i)
	}
}

func TestValidateEmptyJsonpath(t *testing.T) {
	rule := ConfigRule{
		Name:     "TestValidateEmptyJsonpath",
		Jsonpath: "",
		Regexp:   ".*",
	}
	validator := NewValidator()
	if err := validator.AddRule("Foo", rule); err == nil {
		t.Errorf("Adding rule should fail")
	}
	if err := validator.Validate("TestValidateEmptyJsonpath", "Foo", `{"foo": 0}`); err != nil {
		t.Errorf("Validation of empty rule should pass: %s", err)
	}
}

func TestValidateNoRegexp(t *testing.T) {
	rule := ConfigRule{
		Name:     "TestValidateNoRegexp",
		Jsonpath: "{.apiVersion}",
	}
	validator := NewValidator()
	if err := validator.AddRule("Foo", rule); err != nil {
		t.Errorf("Validator shouldn't fail adding rule: %s", err)
	}
	var object map[string]string
	if err := json.Unmarshal([]byte(`{"apiVersion": "v1"}`), &object); err != nil {
		t.Errorf("Deserializing should not fail")
	}

	if err := validator.Validate("TestValidateNoRegexp", "Foo", object); err == nil {
		t.Errorf("Validating object wihtout regexp should fail")
	}
}

func TestValidateRejectRegexpMatch(t *testing.T) {
	rule := ConfigRule{
		Name:     "TestValidateRejectRegexpMatch",
		Jsonpath: "{.apiVersion}",
		Regexp:   "foo",
	}
	validator := NewValidator()
	if err := validator.AddRule("Foo", rule); err != nil {
		t.Errorf("Validator shouldn't fail adding rule: %s", err)
	}
	var object map[string]string
	if err := json.Unmarshal([]byte(`{"apiVersion": "foobarbaz"}`), &object); err != nil {
		t.Errorf("Deserializing should not fail")
	}

	if err := validator.Validate("TestValidateRejectRegexpMatch", "Foo", object); err == nil {
		t.Errorf("Validating object matching regexp should fail")
	}
}

func TestValidateRejectMultipleValues(t *testing.T) {
	rule := ConfigRule{
		Name:     "TestValidateRejectMultipleValues",
		Jsonpath: "{.metadata['name','version']}",
		Regexp:   "foo v1",
	}
	validator := NewValidator()
	if err := validator.AddRule("Foo", rule); err != nil {
		t.Errorf("Validator shouldn't fail adding rule: %s", err)
	}
	var object map[string]interface{}
	if err := json.Unmarshal([]byte(`{"metadata":{"name":"foo","version":"v1"}}`), &object); err != nil {
		t.Errorf("Deserializing should not fail")
	}

	if err := validator.Validate("TestValidateRejectMultipleValues", "Foo", object); err == nil {
		t.Errorf("Validating object matching multiple values with regexp should fail")
	}
}

func TestValidateRejectMultipleRules(t *testing.T) {
	rule1 := ConfigRule{
		Name:     "TestValidateRejectMultipleRules1",
		Jsonpath: "{.metadata.name}",
		Regexp:   "foo",
	}
	rule2 := ConfigRule{
		Name:     "TestValidateRejectMultipleRules2",
		Jsonpath: "{.metadata.version}",
		Regexp:   "v1",
	}
	validator := NewValidator()
	if err := validator.AddRule("Foo", rule1); err != nil {
		t.Errorf("Validator shouldn't fail adding rule1")
	}
	if err := validator.AddRule("Foo", rule2); err != nil {
		t.Errorf("Validator shouldn't fail adding rule2")
	}
	var object map[string]interface{}
	if err := json.Unmarshal([]byte(`{"metadata":{"name":"foo","version":"v1"}}`), &object); err != nil {
		t.Errorf("Deserializing should not fail")
	}

	if err := validator.Validate("TestValidateRejectMultipleRules", "Foo", object); err == nil {
		t.Errorf("Validating object with multiple rules should fail")
	}
}

func TestValidateRejectUnwantedLabel(t *testing.T) {
	rule := ConfigRule{
		Name:     "TestValidateRejectUnwantedLabel",
		Jsonpath: "{.metadata.labels.foo}",
		Regexp:   ".*",
	}
	validator := NewValidator()
	if err := validator.AddRule("Foo", rule); err != nil {
		t.Errorf("Validator shouldn't fail adding rule: %s", err)
	}
	var object map[string]interface{}
	if err := json.Unmarshal([]byte(`{"metadata":{"labels":{"foo":"bar"}}}`), &object); err != nil {
		t.Errorf("Deserializing should not fail")
	}

	if err := validator.Validate("TestValidateRejectUnwantedLabel", "Foo", object); err == nil {
		t.Errorf("Validating object for unwanted label should fail")
	}
}

func TestValidateRejectMissingLabel(t *testing.T) {
	rule := ConfigRule{
		Name:     "TestValidateRejectMissingLabel",
		Jsonpath: "{.metadata.labels.foo}",
		Regexp:   "^$",
	}
	validator := NewValidator()
	if err := validator.AddRule("Foo", rule); err != nil {
		t.Errorf("Validator shouldn't fail adding rule: %s", err)
	}
	var object map[string]interface{}
	if err := json.Unmarshal([]byte(`{"metadata":{"labels":{"baz":"bar"}}}`), &object); err != nil {
		t.Errorf("Deserializing should not fail")
	}

	if err := validator.Validate("TestValidateRejectMissingLabel", "Foo", object); err == nil {
		t.Errorf("Validating object with missing required label should fail")
	}
}

func TestValidateAcceptRequiredLabel(t *testing.T) {
	rule := ConfigRule{
		Name:     "TestValidateAcceptRequiredLabel",
		Jsonpath: "{.metadata.labels.foo}",
		Regexp:   "^$",
	}
	validator := NewValidator()
	if err := validator.AddRule("Foo", rule); err != nil {
		t.Errorf("Validator shouldn't fail adding rule: %s", err)
	}
	var object map[string]interface{}
	if err := json.Unmarshal([]byte(`{"metadata":{"labels":{"foo":"bar"}}}`), &object); err != nil {
		t.Errorf("Deserializing should not fail")
	}

	if err := validator.Validate("TestValidateAcceptRequiredLabel", "Foo", object); err != nil {
		t.Errorf("Validating object with present required label should pass")
	}
}

func TestValidateShouldReturnMessage(t *testing.T) {
	rule := ConfigRule{
		Name:     "TestValidateShouldReturnMessage",
		Jsonpath: "{.metadata.labels.foo}",
		Regexp:   "^$",
		Message:  "Error message",
	}
	validator := NewValidator()
	if err := validator.AddRule("Foo", rule); err != nil {
		t.Errorf("Validator shouldn't fail adding rule: %s", err)
	}
	var object map[string]interface{}
	if err := json.Unmarshal([]byte(`{"metadata":{"labels":{"baz":"bar"}}}`), &object); err != nil {
		t.Errorf("Deserializing should not fail")
	}

	if err := validator.Validate("TestValidateShouldReturnMessage", "Foo", object); err.Error() != "Error message" {
		t.Errorf("Rejected object should return defined error message. Expected: 'Error message', got: '%s'", err)
	}
}

func TestValidateShouldReturnMessagesJoined(t *testing.T) {
	rule1 := ConfigRule{
		Name:     "TestValidateShouldReturnMessagesJoined1",
		Jsonpath: "{.metadata.labels.foo}",
		Regexp:   "^$",
		Message:  "Label foo missing",
	}
	rule2 := ConfigRule{
		Name:     "TestValidateShouldReturnMessagesJoined2",
		Jsonpath: "{.metadata.labels.bar}",
		Regexp:   "^$",
		Message:  "Label bar missing",
	}
	validator := NewValidator()
	if err := validator.AddRule("Foo", rule1); err != nil {
		t.Errorf("Validator shouldn't fail adding rule1")
	}
	if err := validator.AddRule("Foo", rule2); err != nil {
		t.Errorf("Validator shouldn't fail adding rule2")
	}
	var object map[string]interface{}
	if err := json.Unmarshal([]byte(`{"metadata":{"labels":{"baz":"bar"}}}`), &object); err != nil {
		t.Errorf("Deserializing should not fail")
	}

	if err := validator.Validate("TestValidateShouldReturnMessagesJoined", "Foo", object); err.Error() != "Label foo missing, Label bar missing" {
		t.Errorf("Rejected object should return defined error messages. Expected: 'Label foo missing, Label bar missing', got: '%s'", err)
	}
}
