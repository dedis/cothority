import os, sys
sys.path.append(os.path.join(os.path.dirname(__file__), ".."))

import asyncio
import websockets

from skipchain import skipchain_pb2

async def getBlocksAsync(url, block_start, block_end=b'', max_height=1):
    async with websockets.connect(url + '/Skipchain/GetBlocks') as websocket:
        request = skipchain_pb2.GetBlocksRequest()
        request.Start = block_start
        request.End = block_end
        request.MaxHeight = max_height
        await websocket.send(request.SerializeToString())

        reply = await websocket.recv()
        block_reply = skipchain_pb2.GetBlocksResponse()
        block_reply.ParseFromString(reply)
        return block_reply

def getBlocks(url, block_start, block_end=b'', max_height=1):
    if block_end is not b'':
        block_end = bytes.fromhex(block_end)
    if block_start is not b'':
        block_start = bytes.fromhex(block_start)
    return asyncio.get_event_loop().run_until_complete(getBlocksAsync(url, block_start, block_end, max_height))


async def storeBlockAsync(url, block):
    async with websockets.connect(url + '/Skipchain/StoreSkipBlock') as websocket:
        request = skipchain_pb2.StoreSkipBlockRequest()
        request.NewBlock.CopyFrom(block)
        await websocket.send(request.SerializeToString())

        reply = await websocket.recv()
        block_reply = skipchain_pb2.StoreSkipBlockResponse()
        block_reply.ParseFromString(reply)
        return block_reply

def storeBlock(url, block):
    return asyncio.get_event_loop().run_until_complete(storeBlockAsync(url, block))


def createNextBlock(last, data):
    block = skipchain_pb2.SkipBlock()
    if last.GenesisID == b'':
        block.GenesisID = last.Hash
    else:
        block.GenesisID = last.GenesisID
    block.Data = data
    block.Index = last.Index + 1
    block.Roster.CopyFrom(last.Roster)
    return block

