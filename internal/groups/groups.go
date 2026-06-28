package groups

import (
	"fmt"
	"os/user"
)

func IsUserIn(groups ...string) bool {
	//add root as an allowed group
	groups = append(groups, "root")

	u, err := user.Current()
	if err != nil {
		fmt.Println(err.Error())
		return false
	}

	allowedIDs := make([]string, 0)
	for _, v := range groups {
		rootGroup, err := user.LookupGroup(v)
		if err != nil {
			fmt.Println(err.Error())
		} else {
			allowedIDs = append(allowedIDs, rootGroup.Gid)
		}
	}

	g, err := u.GroupIds()
	if err != nil {
		fmt.Println(err.Error())
		return false
	}

	for _, v := range g {
		for _, t := range allowedIDs {
			if v == t {
				return true
			}
		}
	}

	return false
}
