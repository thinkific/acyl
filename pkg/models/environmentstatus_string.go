// Code generated by "stringer -type=EnvironmentStatus"; DO NOT EDIT.

package models

import "strconv"

const _EnvironmentStatus_name = "UnknownStatusSpawnedSuccessFailureDestroyedUpdating"

var _EnvironmentStatus_index = [...]uint8{0, 13, 20, 27, 34, 43, 51}

func (i EnvironmentStatus) String() string {
	if i < 0 || i >= EnvironmentStatus(len(_EnvironmentStatus_index)-1) {
		return "EnvironmentStatus(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _EnvironmentStatus_name[_EnvironmentStatus_index[i]:_EnvironmentStatus_index[i+1]]
}
