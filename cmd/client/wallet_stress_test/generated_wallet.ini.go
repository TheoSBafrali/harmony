package main

const (
	defaultWalletIni = `[default]
bootnode = /ip4/100.26.90.187/tcp/9874/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv
bootnode = /ip4/54.213.43.194/tcp/9874/p2p/QmZJJx6AdaoEkGLrYG4JeLCKeCKDjnFz2wfHNHxAqFSGA9
bootnode = /ip4/13.113.101.219/tcp/12019/p2p/QmQayinFSgMMw5cSpDUiD9pQ2WeP6WNmGxpZ6ou3mdVFJX
bootnode = /ip4/99.81.170.167/tcp/12019/p2p/QmRVbTpEYup8dSaURZfF6ByrMTSKa4UyUzJhSjahFzRqNj
shards = 4

[default.shard0.rpc]
rpc = l0.t.hmny.io:14555
rpc = s0.t.hmny.io:14555

[default.shard1.rpc]
rpc = l1.t.hmny.io:14555
rpc = s1.t.hmny.io:14555

[default.shard2.rpc]
rpc = l2.t.hmny.io:14555
rpc = s2.t.hmny.io:14555

[default.shard3.rpc]
rpc = l3.t.hmny.io:14555
rpc = s3.t.hmny.io:14555

[local]
bootnode = /ip4/127.0.0.1/tcp/19876/p2p/Qmc1V6W7BwX8Ugb42Ti8RnXF1rY5PF7nnZ6bKBryCgi6cv
shards = 2

[local.shard0.rpc]
rpc = 127.0.0.1:14555
rpc = 127.0.0.1:14557
rpc = 127.0.0.1:14559

[local.shard1.rpc]
rpc = 127.0.0.1:14556
rpc = 127.0.0.1:14558
rpc = 127.0.0.1:14560

[beta]
bootnode = /ip4/54.213.43.194/tcp/9868/p2p/QmZJJx6AdaoEkGLrYG4JeLCKeCKDjnFz2wfHNHxAqFSGA9
bootnode = /ip4/100.26.90.187/tcp/9868/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv
bootnode = /ip4/13.113.101.219/tcp/12018/p2p/QmQayinFSgMMw5cSpDUiD9pQ2WeP6WNmGxpZ6ou3mdVFJX
shards = 2

[beta.shard0.rpc]
rpc = l0.b.hmny.io:14555
rpc = s0.b.hmny.io:14555

[beta.shard1.rpc]
rpc = l1.b.hmny.io:14555
rpc = s1.b.hmny.io:14555

[pangaea]
bootnode = /ip4/54.86.126.90/tcp/9867/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv
bootnode = /ip4/52.40.84.2/tcp/9867/p2p/QmZJJx6AdaoEkGLrYG4JeLCKeCKDjnFz2wfHNHxAqFSGA9
shards = 4

[pangaea.shard0.rpc]
rpc = l0.pga.hmny.io:14555
rpc = s0.pga.hmny.io:14555

[pangaea.shard1.rpc]
rpc = l1.pga.hmny.io:14555
rpc = s1.pga.hmny.io:14555

[pangaea.shard2.rpc]
rpc = l2.pga.hmny.io:14555
rpc = s2.pga.hmny.io:14555

[pangaea.shard3.rpc]
rpc = l3.pga.hmny.io:14555
rpc = s3.pga.hmny.io:14555
`
)
