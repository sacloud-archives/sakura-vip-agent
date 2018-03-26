# sakura-vip-agent

[![Go Report Card](https://goreportcard.com/badge/github.com/sacloud/sakura-vip-agent)](https://goreportcard.com/report/github.com/sacloud/sakura-vip-agent)
[![Build Status](https://travis-ci.org/sacloud/sakura-vip-agent.svg?branch=master)](https://travis-ci.org/sacloud/sakura-vip-agent)

`sakura-vip-agent` is agent to handling VIP for DSR-LoadBalancer running on kubernetes cluster.

It is running on each node as `DaemonSet` and assigns a virtual IP to the loopback interface of the running node 
when the service of `type: LoadBalancer` is created and ExternalIP is assigned.

## Usage

    $ kubectl create -f examples/sakura-vip-agent-daemonset.yaml

## License

 `sakura-vip-agent` Copyright (C) 2018 Kazumichi Yamamoto.

  This project is published under [Apache 2.0 License](LICENSE.txt).
  
## Author

  * Kazumichi Yamamoto ([@yamamoto-febc](https://github.com/yamamoto-febc))
