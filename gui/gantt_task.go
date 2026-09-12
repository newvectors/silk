package gui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// GanttTask is a calendar-day task. End is exclusive; a milestone has
// Start == End. Dependencies contain finish-to-start predecessor IDs.
// Parent dates/progress are derived from their leaves, not independently set.
type GanttTask struct {
	ID           string    `json:"id"`
	ParentID     string    `json:"parentId,omitempty"`
	Name         string    `json:"name"`
	Start        time.Time `json:"start"`
	End          time.Time `json:"end"`
	Progress     float64   `json:"progress,omitempty"`
	Owner        string    `json:"owner,omitempty"`
	Description  string    `json:"description,omitempty"`
	Dependencies []string  `json:"dependencies,omitempty"`
	Milestone    bool      `json:"milestone,omitempty"`
	Collapsed    bool      `json:"collapsed,omitempty"`
}

type ganttTaskJSON struct {
	ID           string   `json:"id"`
	ParentID     string   `json:"parentId,omitempty"`
	Name         string   `json:"name"`
	Start        string   `json:"start"`
	End          string   `json:"end"`
	Progress     float64  `json:"progress,omitempty"`
	Owner        string   `json:"owner,omitempty"`
	Description  string   `json:"description,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
	Milestone    bool     `json:"milestone,omitempty"`
	Collapsed    bool     `json:"collapsed,omitempty"`
}

// GanttDate keeps the input's civil date, not its instant in UTC. This makes
// one-day drags stable across DST, offsets and JSON round trips.
func GanttDate(t time.Time) time.Time {
	if t.IsZero() {
		return time.Time{}
	}
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// GanttDaysBetween returns calendar days, without time.Duration's ~290-year
// range limit. Both dates are interpreted in their respective civil calendar.
func GanttDaysBetween(start, end time.Time) int {
	return int((GanttDate(end).Unix() - GanttDate(start).Unix()) / 86400)
}

func ganttDateString(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return GanttDate(t).Format(time.DateOnly)
}

func (task GanttTask) MarshalJSON() ([]byte, error) {
	return json.Marshal(ganttTaskJSON{task.ID, task.ParentID, task.Name,
		ganttDateString(task.Start), ganttDateString(task.End), task.Progress,
		task.Owner, task.Description, task.Dependencies, task.Milestone, task.Collapsed})
}

func (task *GanttTask) UnmarshalJSON(data []byte) error {
	var wire ganttTaskJSON
	if err := ganttDecodeJSON(data, &wire); err != nil {
		return err
	}
	parse := func(s string) (time.Time, error) {
		if s == "" {
			return time.Time{}, nil // only derived parent dates may be omitted
		}
		date, err := time.Parse(time.DateOnly, s)
		if err != nil {
			date, err = time.Parse(time.RFC3339Nano, s)
		}
		if err != nil {
			return time.Time{}, fmt.Errorf("甘特任务 %q 的日期 %q 无效", wire.ID, s)
		}
		return GanttDate(date), nil
	}
	start, err := parse(wire.Start)
	if err != nil {
		return err
	}
	end, err := parse(wire.End)
	if err != nil {
		return err
	}
	*task = GanttTask{wire.ID, wire.ParentID, wire.Name, start, end, wire.Progress,
		wire.Owner, wire.Description, append([]string(nil), wire.Dependencies...), wire.Milestone, wire.Collapsed}
	return nil
}

const ganttMaxJSONBytes = 4 << 20

// Reject unknown fields, duplicate keys, trailing values and excessive nesting.
// IDs encode the task hierarchy, so deep task nesting never needs deep JSON.
func ganttDecodeJSON(data []byte, target any) error {
	if len(data) > ganttMaxJSONBytes {
		return fmt.Errorf("甘特 JSON 超过 4 MiB 限制")
	}
	scan := json.NewDecoder(bytes.NewReader(data))
	scan.UseNumber()
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 16 {
			return fmt.Errorf("甘特 JSON 嵌套过深")
		}
		token, err := scan.Token()
		if err != nil {
			return err
		}
		delim, compound := token.(json.Delim)
		if !compound {
			return nil
		}
		switch delim {
		case '{':
			seen := make(map[string]bool)
			for scan.More() {
				key, err := scan.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				// encoding/json matches struct fields case-insensitively too;
				// reject id/ID as one duplicate rather than silently taking last.
				folded := strings.ToLower(name)
				if !ok || seen[folded] {
					return fmt.Errorf("甘特 JSON 包含重复或无效字段 %q", name)
				}
				seen[folded] = true
				if err := walk(depth + 1); err != nil {
					return err
				}
			}
		case '[':
			for scan.More() {
				if err := walk(depth + 1); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("甘特 JSON 分隔符无效")
		}
		_, err = scan.Token()
		return err
	}
	if err := walk(0); err != nil {
		return err
	}
	if _, err := scan.Token(); err != io.EOF {
		return fmt.Errorf("甘特 JSON 必须只包含一个值")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}
