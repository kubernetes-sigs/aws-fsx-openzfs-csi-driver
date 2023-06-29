# Driver Options
There are several driver options that may be passed as arguments when deploying the driver.

| Option argument             | value sample                                      | default                                             | Description                                             |
|-----------------------------|---------------------------------------------------|-----------------------------------------------------|---------------------------------------------------------|
| endpoint                    | tcp://127.0.0.1:10000/                            | unix:///var/lib/csi/sockets/pluginproxy/csi.sock    | The socket on which the driver will listen for CSI RPCs |
| logging-format              | json                                              | text                                                | Sets the log format. Permitted formats: text, json      |
