<img src="https://photos.puppyspot.com/0/listing/636920/photo/5438753_large-resize.jpg"/>

## Miska Remote File System

MiskaRFS is a light-weight, secured remote file system framework that provides a set of APIs for both the hosting machine and remote clients. Characteristics of this system including: 1) no port forwarding needed, 2) security with PAKE encryption, 3) dynamically add/remove remote commands, 3) file system protection via invisibleFiles and readOnly

1. No Port Forwarding
    - Traditional network programs require port forwarding to connect from the client to the host. In MiskaRFS, communication between host and client is realized through a relay server that serve as the middleman. Connection with the relay is secured. Information flow from host to client can have further security attributes.

2. Security
    - PAKE encryption is utilized to provide safe connections with host/client

3. Add/Remove Commands
    - Interaction between host and client is via commands. MiskaRFS provides a set of APIs for host to customize their exported functionalities in go function.

4. File System Protection
    - MiskaRFS enforces strict file protection protocol. Client may only view files/dirs under the given baseDir name that host provides. In addition, host may make the file system as ReadOnly for remote view of local files.

## Coming
1. Fast + Secured Upload/Download
