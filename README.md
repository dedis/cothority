# cothorities

This is a testing-framework for different projects from dedis/prifi-tree. The goal is to have a structure that permits
different platforms and tests to run.

Platforms:
    * Deter - running
    * Go-routines - should be running
    * Future:
        * Docker
        * LXC

Tests
    * sign
    * stamp
    * vote
    
Life of simulation:
1. Setup
    * read configuration
    * compile eventual files
2. Distribute
    * make sure the environment is up and running
    * copy files
3. Run simulation
    * start all logservers
    * start all nodes
    * start all clients
4. Stop simulation
    * abort after timeout OR
    * wait for final message
5. Collect data
    * copy everything to local