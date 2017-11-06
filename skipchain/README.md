# SkipChains

The DEDIS-team is proud to present our very own BFT-blockchain called SkipChains.
It's a permissioned blockchain with backward- and forward-links, and additional
 higher-level links for faster traversal of the chain.
Our main goal is to have a structure that is lightweight and can still prove 
 that a given block is part of the chain.
Instead of doing Proof-of-Work, every skipchain can define its own verification-
 function that is run on all nodes. 
If 2/3 or more of the nodes accept the new block, it is collectively signed
 and appended to the blockchain.
The decision on the signature is done using a BFT-algorithm with our home-grown
 BFT-CoSi [http://arxiv.org/abs/1602.06997].
