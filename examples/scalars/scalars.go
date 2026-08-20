package scalars
import (
	"encoding/json"
	"strconv"
	"time"
)

// IntTime 是一个自定义 Scalar，在业务逻辑中作为 time.Time 使用，
// 在 API 传输层作为 int64 (秒级时间戳) 或 string (用于 Path/Query/Header) 使用。
type IntTime time.Time

// FromParam 处理 Path/Query/Header 参数 (string -> time.Time)
func (it *IntTime) FromParam(ctx any, s string) error {
	if s == "" {
		return nil
	}
	sec, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*it = IntTime(time.Unix(sec, 0))
	return nil
}

// FromValue 处理 Body 参数 (int64 -> time.Time)
func (it *IntTime) FromValue(ctx any, v int64) error {
	*it = IntTime(time.Unix(v, 0))
	return nil
}

// ToValue 处理 Response 序列化 (time.Time -> int64)
func (it IntTime) ToValue(ctx any) (int64, error) {
	return time.Time(it).Unix(), nil
}

// Std 返回标准的 time.Time
func (it IntTime) Std() time.Time {
	return time.Time(it)
}

// String 实现 Stringer 接口
func (it IntTime) String() string {
	return time.Time(it).Format(time.RFC3339)
}

// MarshalJSON 实现 json.Marshaler 接口。
func (it IntTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(it).Unix())
}

// UnmarshalJSON 实现 json.Unmarshaler 接口。
func (it *IntTime) UnmarshalJSON(data []byte) error {
	var sec int64
	if err := json.Unmarshal(data, &sec); err != nil {
		return err
	}
	*it = IntTime(time.Unix(sec, 0))
	return nil
}


