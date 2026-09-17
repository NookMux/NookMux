package common

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	modelconfig "github.com/NookMux/NookMux/internal/config/model"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var negativeIndexRegexp = regexp.MustCompile(`\.(-\d+)`)

const (
	paramOverrideContextRequestHeaders = "request_headers"
	paramOverrideContextHeaderOverride = "header_override"
)

type ConditionOperation struct {
	Path           string      `json:"path"`             // JSON路径
	Mode           string      `json:"mode"`             // full, prefix, suffix, contains, gt, gte, lt, lte
	Value          interface{} `json:"value"`            // 匹配的值
	Invert         bool        `json:"invert"`           // 反选功能，true表示取反结果
	PassMissingKey bool        `json:"pass_missing_key"` // 未获取到json key时的行为
}

type ParamOperation struct {
	Path       string               `json:"path"`
	Mode       string               `json:"mode"` // delete, set, move, copy, prepend, append, trim_prefix, trim_suffix, ensure_prefix, ensure_suffix, trim_space, to_lower, to_upper, replace, regex_replace, return_error, prune_objects, set_header, delete_header, copy_header, move_header, pass_headers, sync_fields
	Value      interface{}          `json:"value"`
	KeepOrigin bool                 `json:"keep_origin"`
	From       string               `json:"from,omitempty"`
	To         string               `json:"to,omitempty"`
	Conditions []ConditionOperation `json:"conditions,omitempty"` // 条件列表
	Logic      string               `json:"logic,omitempty"`      // AND, OR (默认OR)
}

type ParamOverrideReturnError struct {
	Message    string
	StatusCode int
	Code       string
	Type       string
	SkipRetry  bool
}

func (err *ParamOverrideReturnError) Error() string {
	if err == nil || strings.TrimSpace(err.Message) == "" {
		return "request blocked by param override"
	}
	return err.Message
}

func ApplyParamOverride(jsonData []byte, paramOverride map[string]interface{}, conditionContext map[string]interface{}) ([]byte, error) {
	if len(paramOverride) == 0 {
		return jsonData, nil
	}

	// 尝试断言为操作格式
	if operations, ok := tryParseOperations(paramOverride); ok {
		legacyOverride := buildLegacyParamOverride(paramOverride)
		workingJSON := jsonData
		var err error
		if len(legacyOverride) > 0 {
			workingJSON, err = applyOperationsLegacy(workingJSON, legacyOverride)
			if err != nil {
				return nil, err
			}
		}
		return applyOperations(workingJSON, operations, conditionContext)
	}

	// 直接使用旧方法
	return applyOperationsLegacy(jsonData, paramOverride)
}

func buildLegacyParamOverride(paramOverride map[string]interface{}) map[string]interface{} {
	if len(paramOverride) == 0 {
		return nil
	}
	legacyOverride := make(map[string]interface{}, len(paramOverride))
	for key, value := range paramOverride {
		if strings.EqualFold(strings.TrimSpace(key), "operations") {
			continue
		}
		legacyOverride[key] = value
	}
	return legacyOverride
}

func ApplyParamOverrideWithRelayInfo(jsonData []byte, info *RelayInfo) ([]byte, error) {
	if info == nil || info.ChannelMeta == nil || len(info.ChannelMeta.ParamOverride) == 0 {
		return jsonData, nil
	}

	conditionContext := BuildParamOverrideContext(info)
	result, err := ApplyParamOverride(jsonData, info.ChannelMeta.ParamOverride, conditionContext)
	if err != nil {
		return nil, err
	}
	syncRuntimeHeaderOverrideFromContext(info, conditionContext)
	return result, nil
}

func tryParseOperations(paramOverride map[string]interface{}) ([]ParamOperation, bool) {
	// 检查是否包含 "operations" 字段
	if opsValue, exists := paramOverride["operations"]; exists {
		if opsSlice, ok := opsValue.([]interface{}); ok {
			var operations []ParamOperation
			for _, op := range opsSlice {
				if opMap, ok := op.(map[string]interface{}); ok {
					operation := ParamOperation{}

					// 断言必要字段
					if path, ok := opMap["path"].(string); ok {
						operation.Path = path
					}
					if mode, ok := opMap["mode"].(string); ok {
						operation.Mode = mode
					} else {
						return nil, false // mode 是必需的
					}

					// 可选字段
					if value, exists := opMap["value"]; exists {
						operation.Value = value
					}
					if keepOrigin, ok := opMap["keep_origin"].(bool); ok {
						operation.KeepOrigin = keepOrigin
					}
					if from, ok := opMap["from"].(string); ok {
						operation.From = from
					}
					if to, ok := opMap["to"].(string); ok {
						operation.To = to
					}
					if logic, ok := opMap["logic"].(string); ok {
						operation.Logic = logic
					} else {
						operation.Logic = "OR" // 默认为OR
					}

					// 解析条件
					if conditions, exists := opMap["conditions"]; exists {
						if condSlice, ok := conditions.([]interface{}); ok {
							for _, cond := range condSlice {
								if condMap, ok := cond.(map[string]interface{}); ok {
									condition := ConditionOperation{}
									if path, ok := condMap["path"].(string); ok {
										condition.Path = path
									}
									if mode, ok := condMap["mode"].(string); ok {
										condition.Mode = mode
									}
									if value, ok := condMap["value"]; ok {
										condition.Value = value
									}
									if invert, ok := condMap["invert"].(bool); ok {
										condition.Invert = invert
									}
									if passMissingKey, ok := condMap["pass_missing_key"].(bool); ok {
										condition.PassMissingKey = passMissingKey
									}
									operation.Conditions = append(operation.Conditions, condition)
								}
							}
						}
					}

					operations = append(operations, operation)
				} else {
					return nil, false
				}
			}
			return operations, true
		}
	}

	return nil, false
}

func checkConditions(data []byte, contextJSON string, conditions []ConditionOperation, logic string) (bool, error) {
	if len(conditions) == 0 {
		return true, nil // 没有条件，直接通过
	}
	results := make([]bool, len(conditions))
	for i, condition := range conditions {
		result, err := checkSingleCondition(data, contextJSON, condition)
		if err != nil {
			return false, err
		}
		results[i] = result
	}

	if strings.ToUpper(logic) == "AND" {
		for _, result := range results {
			if !result {
				return false, nil
			}
		}
		return true, nil
	} else {
		for _, result := range results {
			if result {
				return true, nil
			}
		}
		return false, nil
	}
}

func checkSingleCondition(data []byte, contextJSON string, condition ConditionOperation) (bool, error) {
	// 处理负数索引
	path := processNegativeIndex(data, condition.Path)
	value := gjson.GetBytes(data, path)
	if !value.Exists() && contextJSON != "" {
		value = gjson.Get(contextJSON, condition.Path)
	}
	if !value.Exists() {
		if condition.PassMissingKey {
			return true, nil
		}
		return false, nil
	}

	// 利用gjson的类型解析
	targetBytes, err := jsonx.Marshal(condition.Value)
	if err != nil {
		return false, fmt.Errorf("failed to marshal condition value: %v", err)
	}
	targetValue := gjson.ParseBytes(targetBytes)

	result, err := compareGjsonValues(value, targetValue, strings.ToLower(condition.Mode))
	if err != nil {
		return false, fmt.Errorf("comparison failed for path %s: %v", condition.Path, err)
	}

	if condition.Invert {
		result = !result
	}
	return result, nil
}

func processNegativeIndex(data []byte, path string) string {
	matches := negativeIndexRegexp.FindAllStringSubmatch(path, -1)

	if len(matches) == 0 {
		return path
	}

	result := path
	for _, match := range matches {
		negIndex := match[1]
		index, _ := strconv.Atoi(negIndex)

		arrayPath := strings.Split(path, negIndex)[0]
		arrayPath = strings.TrimSuffix(arrayPath, ".")

		array := gjson.GetBytes(data, arrayPath)
		if array.IsArray() {
			length := len(array.Array())
			actualIndex := length + index
			if actualIndex >= 0 && actualIndex < length {
				result = strings.Replace(result, match[0], "."+strconv.Itoa(actualIndex), 1)
			}
		}
	}

	return result
}

// compareGjsonValues 直接比较两个gjson.Result，支持所有比较模式
func compareGjsonValues(jsonValue, targetValue gjson.Result, mode string) (bool, error) {
	switch mode {
	case "full":
		return compareEqual(jsonValue, targetValue)
	case "prefix":
		return strings.HasPrefix(jsonValue.String(), targetValue.String()), nil
	case "suffix":
		return strings.HasSuffix(jsonValue.String(), targetValue.String()), nil
	case "contains":
		return strings.Contains(jsonValue.String(), targetValue.String()), nil
	case "gt":
		return compareNumeric(jsonValue, targetValue, "gt")
	case "gte":
		return compareNumeric(jsonValue, targetValue, "gte")
	case "lt":
		return compareNumeric(jsonValue, targetValue, "lt")
	case "lte":
		return compareNumeric(jsonValue, targetValue, "lte")
	default:
		return false, fmt.Errorf("unsupported comparison mode: %s", mode)
	}
}

func compareEqual(jsonValue, targetValue gjson.Result) (bool, error) {
	// 对null值特殊处理：两个都是null返回true，一个是null另一个不是返回false
	if jsonValue.Type == gjson.Null || targetValue.Type == gjson.Null {
		return jsonValue.Type == gjson.Null && targetValue.Type == gjson.Null, nil
	}

	// 对布尔值特殊处理
	if (jsonValue.Type == gjson.True || jsonValue.Type == gjson.False) &&
		(targetValue.Type == gjson.True || targetValue.Type == gjson.False) {
		return jsonValue.Bool() == targetValue.Bool(), nil
	}

	// 如果类型不同，报错
	if jsonValue.Type != targetValue.Type {
		return false, fmt.Errorf("compare for different types, got %v and %v", jsonValue.Type, targetValue.Type)
	}

	switch jsonValue.Type {
	case gjson.True, gjson.False:
		return jsonValue.Bool() == targetValue.Bool(), nil
	case gjson.Number:
		return jsonValue.Num == targetValue.Num, nil
	case gjson.String:
		return jsonValue.String() == targetValue.String(), nil
	default:
		return jsonValue.String() == targetValue.String(), nil
	}
}

func compareNumeric(jsonValue, targetValue gjson.Result, operator string) (bool, error) {
	// 只有数字类型才支持数值比较
	if jsonValue.Type != gjson.Number || targetValue.Type != gjson.Number {
		return false, fmt.Errorf("numeric comparison requires both values to be numbers, got %v and %v", jsonValue.Type, targetValue.Type)
	}

	jsonNum := jsonValue.Num
	targetNum := targetValue.Num

	switch operator {
	case "gt":
		return jsonNum > targetNum, nil
	case "gte":
		return jsonNum >= targetNum, nil
	case "lt":
		return jsonNum < targetNum, nil
	case "lte":
		return jsonNum <= targetNum, nil
	default:
		return false, fmt.Errorf("unsupported numeric operator: %s", operator)
	}
}

// applyOperationsLegacy 原参数覆盖方法
func applyOperationsLegacy(jsonData []byte, paramOverride map[string]interface{}) ([]byte, error) {
	result := jsonData
	var err error
	for key, value := range paramOverride {
		result, err = sjson.SetBytes(result, escapeSJSONPathKey(key), value)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

func escapeSJSONPathKey(key string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `.`, `\.`, `*`, `\*`, `?`, `\?`)
	return replacer.Replace(key)
}

func applyOperations(data []byte, operations []ParamOperation, conditionContext map[string]interface{}) ([]byte, error) {
	context := ensureContextMap(conditionContext)
	result := data
	for _, op := range operations {
		contextJSON, err := marshalContextJSON(context)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal condition context: %v", err)
		}

		ok, err := checkConditions(result, contextJSON, op.Conditions, op.Logic)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}

		opPaths, err := resolveOperationPaths(result, op.Mode, op.Path)
		if err != nil {
			return nil, err
		}

		switch op.Mode {
		case "delete":
			for _, path := range opPaths {
				result, err = sjson.DeleteBytes(result, path)
				if err != nil {
					break
				}
			}
		case "set":
			for _, path := range opPaths {
				if op.KeepOrigin && gjson.GetBytes(result, path).Exists() {
					continue
				}
				result, err = sjson.SetBytes(result, path, op.Value)
				if err != nil {
					break
				}
			}
		case "move":
			opFrom := processNegativeIndex(result, op.From)
			opTo := processNegativeIndex(result, op.To)
			result, err = moveValue(result, opFrom, opTo)
		case "copy":
			if op.From == "" || op.To == "" {
				return nil, fmt.Errorf("copy from/to is required")
			}
			opFrom := processNegativeIndex(result, op.From)
			opTo := processNegativeIndex(result, op.To)
			result, err = copyValue(result, opFrom, opTo)
		case "prepend", "append":
			for _, path := range opPaths {
				result, err = modifyValue(result, path, op.Value, op.KeepOrigin, op.Mode == "prepend")
				if err != nil {
					break
				}
			}
		case "trim_prefix", "trim_suffix":
			for _, path := range opPaths {
				result, err = trimStringValue(result, path, op.Value, op.Mode == "trim_prefix")
				if err != nil {
					break
				}
			}
		case "ensure_prefix", "ensure_suffix":
			for _, path := range opPaths {
				result, err = ensureStringAffix(result, path, op.Value, op.Mode == "ensure_prefix")
				if err != nil {
					break
				}
			}
		case "trim_space":
			for _, path := range opPaths {
				result, err = transformStringValue(result, path, strings.TrimSpace)
				if err != nil {
					break
				}
			}
		case "to_lower":
			for _, path := range opPaths {
				result, err = transformStringValue(result, path, strings.ToLower)
				if err != nil {
					break
				}
			}
		case "to_upper":
			for _, path := range opPaths {
				result, err = transformStringValue(result, path, strings.ToUpper)
				if err != nil {
					break
				}
			}
		case "replace", "regex_replace":
			for _, path := range opPaths {
				if op.Mode == "replace" {
					result, err = replaceStringValue(result, path, op.From, op.To)
				} else {
					result, err = regexReplaceStringValue(result, path, op.From, op.To)
				}
				if err != nil {
					break
				}
			}
		case "return_error":
			returnError, parseErr := parseParamOverrideReturnError(op.Value)
			if parseErr != nil {
				return nil, parseErr
			}
			return nil, returnError
		case "prune_objects":
			for _, path := range opPaths {
				result, err = pruneObjects(result, path, contextJSON, op.Value)
				if err != nil {
					break
				}
			}
		case "set_header":
			err = setHeaderOverrideInContext(context, op.Path, op.Value, op.KeepOrigin)
		case "delete_header":
			err = deleteHeaderOverrideInContext(context, op.Path)
		case "copy_header", "move_header":
			sourceHeader := op.From
			targetHeader := op.To
			if sourceHeader == "" && op.Path != "" {
				sourceHeader = op.Path
			}
			err = copyHeaderOverrideInContext(context, sourceHeader, targetHeader, op.KeepOrigin)
			if err == nil && op.Mode == "move_header" {
				err = deleteHeaderOverrideInContext(context, sourceHeader)
			}
		case "pass_headers":
			headerNames, parseErr := parseHeaderPassThroughNames(op.Value)
			if parseErr != nil {
				return nil, parseErr
			}
			for _, headerName := range headerNames {
				if modelconfig.ShouldRemoveClaudeCodeBillingHeader(headerName) {
					continue
				}
				err = passRequestHeaderToOverride(context, headerName, op.KeepOrigin)
				if err != nil {
					break
				}
			}
		case "sync_fields":
			result, err = syncFieldsBetweenTargets(result, context, op.From, op.To)
		default:
			return nil, fmt.Errorf("unknown operation: %s", op.Mode)
		}
		if err != nil {
			return nil, fmt.Errorf("operation %s failed: %v", op.Mode, err)
		}
	}
	return result, nil
}

func moveValue(data []byte, fromPath, toPath string) ([]byte, error) {
	sourceValue := gjson.GetBytes(data, fromPath)
	if !sourceValue.Exists() {
		return data, fmt.Errorf("source path does not exist: %s", fromPath)
	}
	result, err := sjson.SetBytes(data, toPath, sourceValue.Value())
	if err != nil {
		return nil, err
	}
	return sjson.DeleteBytes(result, fromPath)
}

func copyValue(data []byte, fromPath, toPath string) ([]byte, error) {
	sourceValue := gjson.GetBytes(data, fromPath)
	if !sourceValue.Exists() {
		return data, fmt.Errorf("source path does not exist: %s", fromPath)
	}
	return sjson.SetBytes(data, toPath, sourceValue.Value())
}

func modifyValue(data []byte, path string, value interface{}, keepOrigin, isPrepend bool) ([]byte, error) {
	current := gjson.GetBytes(data, path)
	switch {
	case current.IsArray():
		return modifyArray(data, path, value, isPrepend)
	case current.Type == gjson.String:
		return modifyString(data, path, value, isPrepend)
	case current.Type == gjson.JSON:
		return mergeObjects(data, path, value, keepOrigin)
	}
	return data, fmt.Errorf("operation not supported for type: %v", current.Type)
}

func modifyArray(data []byte, path string, value interface{}, isPrepend bool) ([]byte, error) {
	current := gjson.GetBytes(data, path)
	var newArray []interface{}
	// 添加新值
	addValue := func() {
		if arr, ok := value.([]interface{}); ok {
			newArray = append(newArray, arr...)
		} else {
			newArray = append(newArray, value)
		}
	}
	// 添加原值
	addOriginal := func() {
		current.ForEach(func(_, val gjson.Result) bool {
			newArray = append(newArray, val.Value())
			return true
		})
	}
	if isPrepend {
		addValue()
		addOriginal()
	} else {
		addOriginal()
		addValue()
	}
	return sjson.SetBytes(data, path, newArray)
}

func modifyString(data []byte, path string, value interface{}, isPrepend bool) ([]byte, error) {
	current := gjson.GetBytes(data, path)
	valueStr := fmt.Sprintf("%v", value)
	var newStr string
	if isPrepend {
		newStr = valueStr + current.String()
	} else {
		newStr = current.String() + valueStr
	}
	return sjson.SetBytes(data, path, newStr)
}

func trimStringValue(data []byte, path string, value interface{}, isPrefix bool) ([]byte, error) {
	current := gjson.GetBytes(data, path)
	if current.Type != gjson.String {
		return data, fmt.Errorf("operation not supported for type: %v", current.Type)
	}

	if value == nil {
		return data, fmt.Errorf("trim value is required")
	}
	valueStr := fmt.Sprintf("%v", value)

	var newStr string
	if isPrefix {
		newStr = strings.TrimPrefix(current.String(), valueStr)
	} else {
		newStr = strings.TrimSuffix(current.String(), valueStr)
	}
	return sjson.SetBytes(data, path, newStr)
}

func ensureStringAffix(data []byte, path string, value interface{}, isPrefix bool) ([]byte, error) {
	current := gjson.GetBytes(data, path)
	if current.Type != gjson.String {
		return data, fmt.Errorf("operation not supported for type: %v", current.Type)
	}

	if value == nil {
		return data, fmt.Errorf("ensure value is required")
	}
	valueStr := fmt.Sprintf("%v", value)
	if valueStr == "" {
		return data, fmt.Errorf("ensure value is required")
	}

	currentStr := current.String()
	if isPrefix {
		if strings.HasPrefix(currentStr, valueStr) {
			return data, nil
		}
		return sjson.SetBytes(data, path, valueStr+currentStr)
	}

	if strings.HasSuffix(currentStr, valueStr) {
		return data, nil
	}
	return sjson.SetBytes(data, path, currentStr+valueStr)
}

func transformStringValue(data []byte, path string, transform func(string) string) ([]byte, error) {
	current := gjson.GetBytes(data, path)
	if current.Type != gjson.String {
		return data, fmt.Errorf("operation not supported for type: %v", current.Type)
	}
	return sjson.SetBytes(data, path, transform(current.String()))
}

func replaceStringValue(data []byte, path, from, to string) ([]byte, error) {
	current := gjson.GetBytes(data, path)
	if current.Type != gjson.String {
		return data, fmt.Errorf("operation not supported for type: %v", current.Type)
	}
	if from == "" {
		return data, fmt.Errorf("replace from is required")
	}
	return sjson.SetBytes(data, path, strings.ReplaceAll(current.String(), from, to))
}

func regexReplaceStringValue(data []byte, path, pattern, replacement string) ([]byte, error) {
	current := gjson.GetBytes(data, path)
	if current.Type != gjson.String {
		return data, fmt.Errorf("operation not supported for type: %v", current.Type)
	}
	if pattern == "" {
		return data, fmt.Errorf("regex pattern is required")
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return data, err
	}
	return sjson.SetBytes(data, path, re.ReplaceAllString(current.String(), replacement))
}

func mergeObjects(data []byte, path string, value interface{}, keepOrigin bool) ([]byte, error) {
	current := gjson.GetBytes(data, path)
	var currentMap, newMap map[string]interface{}

	// 解析当前值
	if err := jsonx.UnmarshalJsonStr(current.Raw, &currentMap); err != nil {
		return nil, err
	}
	// 解析新值
	switch v := value.(type) {
	case map[string]interface{}:
		newMap = v
	default:
		jsonBytes, _ := jsonx.Marshal(v)
		if err := jsonx.Unmarshal(jsonBytes, &newMap); err != nil {
			return nil, err
		}
	}
	// 合并
	result := make(map[string]interface{})
	for k, v := range currentMap {
		result[k] = v
	}
	for k, v := range newMap {
		if !keepOrigin || result[k] == nil {
			result[k] = v
		}
	}
	return sjson.SetBytes(data, path, result)
}

func ensureContextMap(conditionContext map[string]interface{}) map[string]interface{} {
	if conditionContext != nil {
		return conditionContext
	}
	return make(map[string]interface{})
}

func parseParamOverrideReturnError(value interface{}) (*ParamOverrideReturnError, error) {
	returnError := &ParamOverrideReturnError{}
	switch typedValue := value.(type) {
	case nil:
		return nil, fmt.Errorf("return_error value is required")
	case string:
		returnError.Message = strings.TrimSpace(typedValue)
	case map[string]interface{}:
		if message, ok := typedValue["message"].(string); ok {
			returnError.Message = strings.TrimSpace(message)
		}
		if code, ok := typedValue["code"].(string); ok {
			returnError.Code = strings.TrimSpace(code)
		}
		if errorType, ok := typedValue["type"].(string); ok {
			returnError.Type = strings.TrimSpace(errorType)
		}
		if skipRetry, ok := typedValue["skip_retry"].(bool); ok {
			returnError.SkipRetry = skipRetry
		}
		if statusCodeRaw, exists := typedValue["status_code"]; exists {
			statusCode, ok := parseOverrideInt(statusCodeRaw)
			if !ok {
				return nil, fmt.Errorf("return_error status_code must be numeric")
			}
			returnError.StatusCode = statusCode
		}
	default:
		return nil, fmt.Errorf("return_error value must be a string or object")
	}
	if strings.TrimSpace(returnError.Message) == "" {
		returnError.Message = "request blocked by param override"
	}
	return returnError, nil
}

func parseOverrideInt(value interface{}) (int, bool) {
	switch typedValue := value.(type) {
	case int:
		return typedValue, true
	case int64:
		return int(typedValue), true
	case float64:
		return int(typedValue), true
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typedValue))
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func marshalContextJSON(context map[string]interface{}) (string, error) {
	if len(context) == 0 {
		return "", nil
	}
	contextBytes, err := jsonx.Marshal(context)
	if err != nil {
		return "", err
	}
	return string(contextBytes), nil
}

func resolveOperationPaths(data []byte, mode string, path string) ([]string, error) {
	if !operationSupportsWildcardPath(mode) {
		if path == "" {
			return nil, nil
		}
		return []string{processNegativeIndex(data, path)}, nil
	}
	if path == "" {
		return nil, nil
	}
	path = processNegativeIndex(data, path)
	if !strings.Contains(path, "*") {
		return []string{path}, nil
	}
	var decoded interface{}
	if err := jsonx.Unmarshal(data, &decoded); err != nil {
		return nil, err
	}
	paths := collectWildcardPaths(decoded, strings.Split(path, "."), nil, mode == "set")
	sort.Strings(paths)
	return paths, nil
}

func operationSupportsWildcardPath(mode string) bool {
	switch mode {
	case "delete", "set", "prepend", "append", "trim_prefix", "trim_suffix", "ensure_prefix", "ensure_suffix", "trim_space", "to_lower", "to_upper", "replace", "regex_replace", "prune_objects":
		return true
	default:
		return false
	}
}

func collectWildcardPaths(value interface{}, segments []string, prefix []string, allowMissingLeaf bool) []string {
	if len(segments) == 0 {
		return []string{strings.Join(prefix, ".")}
	}
	segment := segments[0]
	if segment == "*" {
		return collectWildcardChildPaths(value, segments[1:], prefix, allowMissingLeaf)
	}

	switch typed := value.(type) {
	case map[string]interface{}:
		child, exists := typed[segment]
		if !exists {
			if allowMissingLeaf && len(segments) == 1 {
				return []string{strings.Join(append(prefix, escapeSJSONPathKey(segment)), ".")}
			}
			return nil
		}
		return collectWildcardPaths(child, segments[1:], append(prefix, escapeSJSONPathKey(segment)), allowMissingLeaf)
	case []interface{}:
		index, err := strconv.Atoi(segment)
		if err != nil || index < 0 || index >= len(typed) {
			return nil
		}
		return collectWildcardPaths(typed[index], segments[1:], append(prefix, segment), allowMissingLeaf)
	default:
		return nil
	}
}

func collectWildcardChildPaths(value interface{}, segments []string, prefix []string, allowMissingLeaf bool) []string {
	var paths []string
	switch typed := value.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			paths = append(paths, collectWildcardPaths(typed[key], segments, append(prefix, escapeSJSONPathKey(key)), allowMissingLeaf)...)
		}
	case []interface{}:
		for index, item := range typed {
			paths = append(paths, collectWildcardPaths(item, segments, append(prefix, strconv.Itoa(index)), allowMissingLeaf)...)
		}
	}
	return paths
}

func ensureMapKeyInContext(context map[string]interface{}, key string) map[string]interface{} {
	if context == nil {
		return map[string]interface{}{}
	}
	if existing, ok := context[key].(map[string]interface{}); ok {
		return existing
	}
	converted := make(map[string]interface{})
	if existing, ok := context[key].(map[string]string); ok {
		for headerName, headerValue := range existing {
			converted[headerName] = headerValue
		}
	}
	context[key] = converted
	return converted
}

func normalizeHeaderContextKey(headerName string) string {
	headerName = strings.TrimSpace(headerName)
	if headerName == "" {
		return ""
	}
	return strings.ToLower(headerName)
}

func setHeaderOverrideInContext(context map[string]interface{}, headerName string, value interface{}, keepOrigin bool) error {
	headerName = normalizeHeaderContextKey(headerName)
	if headerName == "" {
		return fmt.Errorf("header name is required")
	}
	headers := ensureMapKeyInContext(context, paramOverrideContextHeaderOverride)
	if keepOrigin {
		if existingValue := strings.TrimSpace(fmt.Sprintf("%v", headers[headerName])); existingValue != "" {
			return nil
		}
	}
	headerValue, includeHeader, err := resolveHeaderOverrideValue(context, headerName, value)
	if err != nil {
		return err
	}
	if !includeHeader {
		delete(headers, headerName)
		return nil
	}
	headers[headerName] = headerValue
	return nil
}

func deleteHeaderOverrideInContext(context map[string]interface{}, headerName string) error {
	headerName = normalizeHeaderContextKey(headerName)
	if headerName == "" {
		return fmt.Errorf("header name is required")
	}
	delete(ensureMapKeyInContext(context, paramOverrideContextHeaderOverride), headerName)
	return nil
}

func copyHeaderOverrideInContext(context map[string]interface{}, sourceHeader string, targetHeader string, keepOrigin bool) error {
	sourceHeader = normalizeHeaderContextKey(sourceHeader)
	targetHeader = normalizeHeaderContextKey(targetHeader)
	if sourceHeader == "" || targetHeader == "" {
		return fmt.Errorf("copy_header source and target header are required")
	}
	sourceValue, exists := getHeaderValueFromContext(context, sourceHeader)
	if !exists {
		return fmt.Errorf("source header does not exist: %s", sourceHeader)
	}
	return setHeaderOverrideInContext(context, targetHeader, sourceValue, keepOrigin)
}

func getHeaderValueFromContext(context map[string]interface{}, headerName string) (string, bool) {
	headerName = normalizeHeaderContextKey(headerName)
	for _, contextKey := range []string{paramOverrideContextHeaderOverride, paramOverrideContextRequestHeaders} {
		if rawHeaders, ok := context[contextKey].(map[string]interface{}); ok {
			for key, value := range rawHeaders {
				if normalizeHeaderContextKey(key) == headerName {
					valueString := strings.TrimSpace(fmt.Sprintf("%v", value))
					return valueString, valueString != ""
				}
			}
		}
	}
	return "", false
}

func resolveHeaderOverrideValue(context map[string]interface{}, headerName string, value interface{}) (string, bool, error) {
	if mapping, ok := value.(map[string]interface{}); ok {
		return mergeHeaderTokenOverrideValue(context, headerName, mapping)
	}
	valueString := strings.TrimSpace(fmt.Sprintf("%v", value))
	if value == nil || valueString == "" {
		return "", false, nil
	}
	return valueString, true, nil
}

func mergeHeaderTokenOverrideValue(context map[string]interface{}, headerName string, mapping map[string]interface{}) (string, bool, error) {
	appendTokens, err := parseHeaderAppendTokens(mapping)
	if err != nil {
		return "", false, err
	}
	keepOnlyDeclared := parseHeaderKeepOnlyDeclared(mapping)
	currentHeaderValue, _ := getHeaderValueFromContext(context, headerName)
	currentTokens := splitHeaderListValue(currentHeaderValue)
	resultTokens := make([]string, 0, len(currentTokens)+len(mapping)+len(appendTokens))

	for _, token := range currentTokens {
		replacementRaw, declared := mapping[token]
		if !declared {
			if !keepOnlyDeclared {
				resultTokens = append(resultTokens, token)
			}
			continue
		}
		replacementTokens, err := parseHeaderReplacementTokens(replacementRaw)
		if err != nil {
			return "", false, err
		}
		resultTokens = append(resultTokens, replacementTokens...)
	}

	resultTokens = append(resultTokens, appendTokens...)
	resultTokens = uniqueStrings(resultTokens)
	if len(resultTokens) == 0 {
		return "", false, nil
	}
	return strings.Join(resultTokens, ","), true, nil
}

func parseHeaderAppendTokens(mapping map[string]interface{}) ([]string, error) {
	appendRaw, ok := mapping["$append"]
	if !ok {
		return nil, nil
	}
	return parseHeaderReplacementTokens(appendRaw)
}

func parseHeaderKeepOnlyDeclared(mapping map[string]interface{}) bool {
	keepOnlyDeclared, ok := mapping["$keep_only_declared"].(bool)
	return ok && keepOnlyDeclared
}

func parseHeaderReplacementTokens(value interface{}) ([]string, error) {
	switch typedValue := value.(type) {
	case nil:
		return nil, nil
	case string:
		return splitHeaderListValue(typedValue), nil
	case []string:
		return splitHeaderListValues(typedValue), nil
	case []interface{}:
		items := make([]string, 0, len(typedValue))
		for _, item := range typedValue {
			items = append(items, fmt.Sprintf("%v", item))
		}
		return splitHeaderListValues(items), nil
	default:
		return nil, fmt.Errorf("unsupported header token value type: %T", value)
	}
}

func splitHeaderListValues(values []string) []string {
	var tokens []string
	for _, value := range values {
		tokens = append(tokens, splitHeaderListValue(value)...)
	}
	return uniqueStrings(tokens)
}

func splitHeaderListValue(value string) []string {
	parts := strings.Split(value, ",")
	tokens := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			tokens = append(tokens, part)
		}
	}
	return tokens
}

func uniqueStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func parseHeaderPassThroughNames(value interface{}) ([]string, error) {
	return parseHeaderReplacementTokens(value)
}

func passRequestHeaderToOverride(context map[string]interface{}, headerName string, keepOrigin bool) error {
	value, exists := getHeaderValueFromSpecificContext(context, paramOverrideContextRequestHeaders, headerName)
	if !exists {
		return nil
	}
	return setHeaderOverrideInContext(context, headerName, value, keepOrigin)
}

func getHeaderValueFromSpecificContext(context map[string]interface{}, contextKey string, headerName string) (string, bool) {
	headerName = normalizeHeaderContextKey(headerName)
	if rawHeaders, ok := context[contextKey].(map[string]interface{}); ok {
		for key, value := range rawHeaders {
			if normalizeHeaderContextKey(key) == headerName {
				valueString := strings.TrimSpace(fmt.Sprintf("%v", value))
				return valueString, valueString != ""
			}
		}
	}
	return "", false
}

type syncTarget struct {
	kind string
	key  string
}

func parseSyncTarget(spec string) (syncTarget, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return syncTarget{}, fmt.Errorf("sync_fields target is required")
	}
	parts := strings.SplitN(spec, ":", 2)
	if len(parts) == 2 && (parts[0] == "json" || parts[0] == "header") {
		return syncTarget{kind: parts[0], key: strings.TrimSpace(parts[1])}, nil
	}
	return syncTarget{kind: "json", key: spec}, nil
}

func readSyncTargetValue(data []byte, context map[string]interface{}, target syncTarget) (interface{}, bool) {
	switch target.kind {
	case "json":
		value := gjson.GetBytes(data, processNegativeIndex(data, target.key))
		if !value.Exists() {
			return nil, false
		}
		return value.Value(), true
	case "header":
		value, exists := getHeaderValueFromContext(context, target.key)
		return value, exists
	default:
		return nil, false
	}
}

func writeSyncTargetValue(data []byte, context map[string]interface{}, target syncTarget, value interface{}) ([]byte, error) {
	switch target.kind {
	case "json":
		return sjson.SetBytes(data, processNegativeIndex(data, target.key), value)
	case "header":
		return data, setHeaderOverrideInContext(context, target.key, value, false)
	default:
		return nil, fmt.Errorf("unsupported sync_fields target kind: %s", target.kind)
	}
}

func syncFieldsBetweenTargets(data []byte, context map[string]interface{}, fromSpec string, toSpec string) ([]byte, error) {
	fromTarget, err := parseSyncTarget(fromSpec)
	if err != nil {
		return nil, err
	}
	toTarget, err := parseSyncTarget(toSpec)
	if err != nil {
		return nil, err
	}
	if _, exists := readSyncTargetValue(data, context, toTarget); exists {
		return data, nil
	}
	value, exists := readSyncTargetValue(data, context, fromTarget)
	if !exists {
		return data, nil
	}
	return writeSyncTargetValue(data, context, toTarget, value)
}

func pruneObjects(data []byte, path string, contextJSON string, value interface{}) ([]byte, error) {
	conditions, err := parsePruneConditions(value)
	if err != nil {
		return nil, err
	}
	if len(conditions) == 0 {
		return data, nil
	}
	current := gjson.GetBytes(data, path)
	if current.IsArray() {
		items := make([]interface{}, 0)
		for _, item := range current.Array() {
			itemBytes := []byte(item.Raw)
			shouldRemove, err := checkConditions(itemBytes, contextJSON, conditions, "AND")
			if err != nil {
				return nil, err
			}
			if !shouldRemove {
				items = append(items, item.Value())
			}
		}
		return sjson.SetBytes(data, path, items)
	}
	if current.Type == gjson.JSON {
		objectValue := make(map[string]interface{})
		if err := jsonx.UnmarshalJsonStr(current.Raw, &objectValue); err != nil {
			return nil, err
		}
		for key, item := range objectValue {
			itemBytes, err := jsonx.Marshal(item)
			if err != nil {
				return nil, err
			}
			shouldRemove, err := checkConditions(itemBytes, contextJSON, conditions, "AND")
			if err != nil {
				return nil, err
			}
			if shouldRemove {
				delete(objectValue, key)
			}
		}
		return sjson.SetBytes(data, path, objectValue)
	}
	return data, fmt.Errorf("prune_objects requires array or object at path: %s", path)
}

func parsePruneConditions(value interface{}) ([]ConditionOperation, error) {
	switch typedValue := value.(type) {
	case []interface{}:
		conditions := make([]ConditionOperation, 0, len(typedValue))
		for _, item := range typedValue {
			conditionMap, ok := item.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("prune_objects condition must be an object")
			}
			conditions = append(conditions, conditionFromMap(conditionMap))
		}
		return conditions, nil
	case map[string]interface{}:
		if rawConditions, ok := typedValue["conditions"].([]interface{}); ok {
			return parsePruneConditions(rawConditions)
		}
		return []ConditionOperation{conditionFromMap(typedValue)}, nil
	default:
		return nil, fmt.Errorf("prune_objects value must be a condition object or list")
	}
}

func conditionFromMap(conditionMap map[string]interface{}) ConditionOperation {
	condition := ConditionOperation{}
	if path, ok := conditionMap["path"].(string); ok {
		condition.Path = path
	}
	if mode, ok := conditionMap["mode"].(string); ok {
		condition.Mode = mode
	}
	if value, exists := conditionMap["value"]; exists {
		condition.Value = value
	}
	if invert, ok := conditionMap["invert"].(bool); ok {
		condition.Invert = invert
	}
	if passMissingKey, ok := conditionMap["pass_missing_key"].(bool); ok {
		condition.PassMissingKey = passMissingKey
	}
	return condition
}

// BuildParamOverrideContext 提供 ApplyParamOverride 可用的上下文信息。
// 目前内置以下字段：
//   - upstream_model/model：始终为通道映射后的上游模型名。
//   - original_model：请求最初指定的模型名。
//   - request_path：请求路径
//   - is_channel_test：是否为渠道测试请求（同 is_test）。
func BuildParamOverrideContext(info *RelayInfo) map[string]interface{} {
	if info == nil {
		return nil
	}

	ctx := make(map[string]interface{})
	ctx[paramOverrideContextHeaderOverride] = cloneStringInterfaceMap(getEffectiveHeaderOverrideMap(info))
	if info.ChannelMeta != nil && info.ChannelMeta.UpstreamModelName != "" {
		ctx["model"] = info.ChannelMeta.UpstreamModelName
		ctx["upstream_model"] = info.ChannelMeta.UpstreamModelName
	}
	if info.OriginModelName != "" {
		ctx["original_model"] = info.OriginModelName
		if _, exists := ctx["model"]; !exists {
			ctx["model"] = info.OriginModelName
		}
	}

	if info.RequestURLPath != "" {
		requestPath := info.RequestURLPath
		if requestPath != "" {
			ctx["request_path"] = requestPath
		}
	}

	ctx["is_channel_test"] = info.IsChannelTest
	return ctx
}

func getEffectiveHeaderOverrideMap(info *RelayInfo) map[string]interface{} {
	if info == nil || info.ChannelMeta == nil {
		return map[string]interface{}{}
	}
	if info.UseRuntimeHeadersOverride {
		return info.RuntimeHeadersOverride
	}
	return info.ChannelMeta.HeadersOverride
}

func GetEffectiveHeaderOverride(info *RelayInfo) map[string]interface{} {
	return cloneStringInterfaceMap(getEffectiveHeaderOverrideMap(info))
}

func syncRuntimeHeaderOverrideFromContext(info *RelayInfo, context map[string]interface{}) {
	if info == nil {
		return
	}
	rawHeaders, ok := context[paramOverrideContextHeaderOverride].(map[string]interface{})
	if !ok {
		return
	}
	info.RuntimeHeadersOverride = cloneStringInterfaceMap(rawHeaders)
	info.UseRuntimeHeadersOverride = true
}

func cloneStringInterfaceMap(source map[string]interface{}) map[string]interface{} {
	if len(source) == 0 {
		return map[string]interface{}{}
	}
	target := make(map[string]interface{}, len(source))
	for key, value := range source {
		target[key] = value
	}
	return target
}
