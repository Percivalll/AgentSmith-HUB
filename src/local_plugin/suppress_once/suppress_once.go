package suppress_once

import (
	"AgentSmith-HUB/common"
	"fmt"
	"strconv"
)

// Eval implements a suppression plugin: for the same key, return true only once
// within the provided time window (seconds). Args:
//
//	0: key string / any comparable value converted to string
//	1: window int (seconds) – suppression period
//	2: ruleid string (optional) – rule identifier to isolate different rules
//
// It uses Redis SETNX with TTL to track fired keys.
func Eval(args ...interface{}) (bool, error) {
	if len(args) < 2 {
		return false, fmt.Errorf("suppressOnce requires at least 2 arguments: key and window(sec), optionally ruleid")
	}
	keyStr := fmt.Sprintf("%v", args[0])

	// parse window seconds
	var winSec int
	switch v := args[1].(type) {
	case int:
		winSec = v
	case int64:
		winSec = int(v)
	case float64:
		winSec = int(v)
	case string:
		i, err := strconv.Atoi(v)
		if err != nil {
			return false, fmt.Errorf("invalid window seconds: %v", v)
		}
		winSec = i
	default:
		return false, fmt.Errorf("unsupported window type %T", v)
	}
	if winSec <= 0 {
		return false, fmt.Errorf("window must be positive seconds")
	}

	// Optional ruleid parameter for rule isolation
	var redisKey string
	if len(args) >= 3 {
		ruleid := fmt.Sprintf("%v", args[2])
		redisKey = "suppress_once:" + ruleid + ":" + keyStr
	} else {
		// Backward compatibility: no ruleid specified
		redisKey = "suppress_once:" + keyStr
	}

	ok, err := common.RedisSetNX(redisKey, 1, winSec)
	if err != nil {
		return false, err
	}
	// ok==true means first time within window → return true; else false
	return ok, nil
}
