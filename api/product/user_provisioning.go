package product

// TODO: this file is defunct and not used... but potentially quite useful...
//     Especially until user provisioning is VERY stable inside of Workbench

import (
	"fmt"
)

var userProvisioningConf = `[program:user-provision]
command=/startup/custom/create-users-ongoing.sh
autorestart=false
numprocs=1
startsecs=10
stdout_logfile=/dev/stdout
stdout_logfile_maxbytes=0
stderr_logfile=/dev/stderr
stderr_logfile_maxbytes=0
`

var createUsersOngoingSh = `#!/bin/bash

cat /startup/custom/users.txt | grep -v '^\s*#' | while read user; do
  USERNAME=$(echo $user | cut -d: -f1)
  UUID=$(echo $user | cut -d: -f2)
  GID=$(echo $user | cut -d: -f3)
  PASSWORD=$(echo $user | cut -d: -f4)
  echo "user-provision: creating user '$USERNAME' with UID '$UUID' and GID '$GID'"
  groupadd -f -g $GID $USERNAME
  useradd -m -u $UUID -g $GID -m -s /bin/bash -N $USERNAME
  if [[ -n "$PASSWORD" ]]; then
    echo "$USERNAME:$PASSWORD" | sudo chpasswd
  fi
done;
echo 'user-provision: sleeping forever'
while :; do read; done
`

type UserToProvision struct {
	Username string
	UID      string
	GID      string
	Password string
}

func createUsersText(users []UserToProvision) string {
	outputText := "# username:uid:gid:password\n"

	for _, u := range users {
		outputText += fmt.Sprintf("%s:%s:%s:%s", u.Username, u.UID, u.GID, u.Password)
		outputText += "\n"
	}

	return outputText
}

func UserProvisioningConfigMapContents(users []UserToProvision) map[string]string {
	return map[string]string{
		"user-provisioning.conf":  userProvisioningConf,
		"create-users-ongoing.sh": createUsersOngoingSh,
		"users.txt":               createUsersText(users),
	}
}
