package contract

import "fmt"

// 唯一汇总公式（计费验收标准"字段定义与唯一汇总公式"）：
//
//	普通输入总量 = text_input + image_input + audio_input + video_input + document_input
//	输入侧总量   = 普通输入总量 + read_cache + write_cache
//	输出总量     = text_output + audio_output + image_output
//	处理总量     = 输入侧总量 + 输出总量
//
// reasoning_output 是 text_output 的子集、accepted/rejected_prediction 只作审计
// 拆分、write_cache_5m/1h 是 write_cache 的子集，均不参与总量相加。
// 旧 logs.prompt_tokens / logs.completion_tokens 列删除后，所有聚合总量
// （wire 投影、TPM、统计）都通过本文件的方法从 billing_details 还原。

// OrdinaryInputTotal 返回普通输入总量（不含缓存读写）。
func (p BillingDetailsPayload) OrdinaryInputTotal() (int, error) {
	return checkedTokenSum(
		p.Tokens.Input.TextInput,
		p.Tokens.Input.ImageInput,
		p.Tokens.Input.AudioInput,
		p.Tokens.Input.VideoInput,
		p.Tokens.Input.DocumentInput,
	)
}

// InputSideTotal 返回输入侧总量（普通输入 + 缓存读取 + 缓存写入）。
// 与旧 prompt_tokens 列语义一致。
func (p BillingDetailsPayload) InputSideTotal() (int, error) {
	ordinary, err := p.OrdinaryInputTotal()
	if err != nil {
		return 0, err
	}
	return checkedTokenSum(ordinary, p.Tokens.Cache.ReadCache, p.Tokens.Cache.WriteCache)
}

// OutputTotal 返回输出总量。与旧 completion_tokens 列语义一致。
func (p BillingDetailsPayload) OutputTotal() (int, error) {
	return checkedTokenSum(
		p.Tokens.Output.TextOutput,
		p.Tokens.Output.AudioOutput,
		p.Tokens.Output.ImageOutput,
	)
}

// ProcessedTotal 返回处理总量（输入侧总量 + 输出总量），即单条消费记录
// 参与 TPM 求和的口径。
func (p BillingDetailsPayload) ProcessedTotal() (int, error) {
	inputSide, err := p.InputSideTotal()
	if err != nil {
		return 0, err
	}
	output, err := p.OutputTotal()
	if err != nil {
		return 0, err
	}
	return checkedTokenSum(inputSide, output)
}

// checkedTokenSum 防溢出求和：payload 经解析端校验后各字段非负，但多字段
// 相加仍可能超出 int 上限，超限时显式报错而不是回绕。
func checkedTokenSum(values ...int) (int, error) {
	const maxInt = int(^uint(0) >> 1)
	sum := 0
	for _, value := range values {
		if value < 0 {
			return 0, fmt.Errorf("negative token detail value %d", value)
		}
		if value > maxInt-sum {
			return 0, fmt.Errorf("token detail sum overflow")
		}
		sum += value
	}
	return sum, nil
}
