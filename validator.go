package main

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/golang/glog"
	jsonpath "k8s.io/client-go/util/jsonpath"
)

// Validator keeps map of supported kinds and their rules
type Validator struct {
	rules map[string][]ValidatorRule
}

// ValidatorRule stores parsed version of ConfigRule
type ValidatorRule struct {
	jsonpath *jsonpath.JSONPath // Parsed JSONPath object
	regexp   *regexp.Regexp     // Compiled Regexp
	message  string             // Error message in case of rejection
	name     string             // Rule name
}

// NewValidator creates new instance of Validator struct
func NewValidator() *Validator {
	rules := make(map[string][]ValidatorRule)
	return &Validator{
		rules: rules,
	}
}

// AddRule parses given ConfigRule's jsonpath and regexp and adds it to validator
func (v *Validator) AddRule(kind string, rule ConfigRule) error {
	glog.Infof("Parsing rule '%s' for kind '%s': JSONPath=%s Regexp=%s", rule.Name, kind, rule.Jsonpath, rule.Regexp)

	if kind == "" {
		return fmt.Errorf("Kind can't be empty")
	}

	if rule.Jsonpath == "" {
		return fmt.Errorf("JSONPath can't be empty")
	}

	if rule.Name == "" {
		return fmt.Errorf("Rule name can't be empty")
	}

	// Create JSONPath object
	jsonpath := jsonpath.New(fmt.Sprintf("%s %s", kind, rule.Name))
	jsonpath.AllowMissingKeys(true)
	if err := jsonpath.Parse(rule.Jsonpath); err != nil {
		return err
	}

	validator_rule := ValidatorRule{
		jsonpath: jsonpath,
		message:  rule.Message,
		name:     rule.Name,
	}

	// Compile regexp
	if rule.Regexp != "" {
		regexp, err := regexp.Compile(rule.Regexp)
		if err != nil {
			return err
		}
		validator_rule.regexp = regexp
	}

	// If everything is fine, append to rules
	v.rules[kind] = append(v.rules[kind], validator_rule)

	return nil
}

// Validate takes object for validation, looks up available validators for given kind and executes them
func (v *Validator) Validate(uid string, kind string, object interface{}) error {
	var errors []string

	// Iterate over all rules we have defined
	for _, rule := range v.rules[kind] {
		buf := new(bytes.Buffer)
		if err := rule.jsonpath.Execute(buf, object); err != nil {
			glog.Errorf("UID=%s Rule=%s: Could not execute JSONPath rule: %v", uid, rule.name, err)
			errors = append(errors, "Failed to validate object")
			continue
		}

		output := buf.String()

		// If regexp is defined and match query output, reject object
		if rule.regexp != nil {
			if rule.regexp.MatchString(output) {
				glog.Infof("UID=%s Rule=%s: Query output matches regexp, rejecting", uid, rule.name)
				errors = append(errors, rule.message)
			}
			continue
		}

		// If regexp is NOT defined but query returned some output, reject object as well
		if output != "" {
			glog.Infof("UID=%s Rule=%s: Query produced output and regexp not defined, rejecting", uid, rule.name)
			errors = append(errors, rule.message)
		}
	}

	// If we found at least one error
	if len(errors) > 0 {
		message := fmt.Errorf(strings.Join(errors, ", "))
		glog.Infof("UID=%s: Found %d reasons to reject: %s", uid, len(errors), message)
		return message
	}

	glog.Infof("UID=%s: No reasons to reject, accepting", uid)
	return nil
}
