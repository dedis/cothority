const shuffle = require("shuffle-array");
import WebSocket from 'isomorphic-ws';
import { Roster } from "./proto";
import { Message, util } from 'protobufjs';

export interface Connection {
    send<T extends Message>(message: Message, reply: typeof Message): Promise<T>;
    getURL(): string;
}

/**
 * Socket is a WebSocket object instance through which protobuf messages are
 * sent to conodes.
 * @param {string} addr websocket address of the conode to contact.
 * @param {string} service name. A socket is tied to a service name.
 *
 * @throws {TypeError} when urlRegistered is not a string or protobuf is not an object
 */
export class WebSocketConnection implements Connection {
    protected url: string;
    private service: string;

    constructor(addr: string, service: string) {
        this.url = addr;
        this.service = service;
    }

    getURL(): string {
        return this.url;
    }

    /**
     * Send transmits data to a given urlRegistered and parses the response.
     * @param {string} request name of registered protobuf message
     * @param {string} response name of registered protobuf message
     * @param {object} data to be sent
     *
     * @returns {object} Promise with response message on success, and an error on failure
     */
    async send<T extends Message>(message: Message, reply: typeof Message): Promise<T> {
        if (!reply.$type) {
            return Promise.reject(new Error('Message is not registered.'));
        }

        return new Promise((resolve, reject) => {
            const path = this.url + "/" + this.service + "/" + message.$type.name.replace(/.*\./, '');
            console.log("Socket: new WebSocket(" + path + ")");
            const ws = new WebSocket(path);
            const bytes = message.$type.encode(message).finish();

            const timerId = setTimeout(() => {
                console.log("timeout - retrying");
                // Not response from the server so we try to send it once more
                ws.send(bytes);
            }, 10000);

            ws.onopen = () => {
                ws.send(bytes);
            };

            ws.onmessage = (evt: any): any => {
                const buf = Buffer.from(evt.data);
                console.log("Getting message with length:", buf.length);

                try {
                    const ret = reply.decode(buf) as T;

                    resolve(ret);
                } catch (err) {
                    if (err instanceof util.ProtocolError) {
                        reject(err);
                    } else {
                        reject(new Error('Error when trying to decode the message'));
                    }
                }

                ws.close(1000);
            };

            ws.onclose = (evt: any) => {
                clearTimeout(timerId);
                if (evt.code !== 1000) {
                    console.log("Got close:", evt.code, evt.reason);
                    reject(new Error(evt.reason));
                }
            };

            ws.onerror = (evt: any) => {
                return console.log("error in websocket: ", evt.error);
            };
        });
    };
}

/*
 * RosterSocket offers similar functionality from the Socket class but chooses
 * a random conode when trying to connect. If a connection fails, it
 * automatically retries to connect to another random server.
 * */
export class RosterWSConnection extends WebSocketConnection {
    addresses: string[];

    constructor(r: Roster, service: string) {
        super('', service);
        this.addresses = r.list.map(conode => conode.getWebSocketAddress());
    }

    /**
     * send tries to send the request to a random server in the list as long as there is no successful response.
     * It tries a permutation of all server's addresses.
     *
     * @param {string} request name of the protobuf packet
     * @param {string} response name of the protobuf packet response
     * @param {Object} data javascript object representing the request
     * @returns {Promise} holds the returned data in case of success.
     */
    async send<T extends Message>(message: Message, reply: typeof Message): Promise<T> {
        const addresses = this.addresses.slice();
        shuffle(addresses);

        for (let i = 0; i < addresses.length; i++) {
            this.url = addresses[i];
            if (this.url == undefined) {
                continue;
            }
            try {
                return super.send(message, reply);
            } catch (err) {
                console.log(err);
            }
        }

        throw new Error("no conodes are available or all conodes returned an error");
    }
}

/**
 * LeaderSocket reads a roster and can be used to communicate with the leader
 * node. As of now the leader is the first node in the roster.
 *
 * @throws {Error} when roster doesn't have any node
 */
export class LeaderConnection extends WebSocketConnection {
    constructor(roster: Roster, service: string) {
        if (roster.list.length === 0) {
            throw new Error("Roster should have at least one node");
        }

        super(roster.list[0].address, service);
    }
}
