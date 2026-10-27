"""TCP proxy for the route integration test, one listener per replica, each of which can be held.

    proxy.py <state dir> <listen ip> <port>=<target ip>:<target port> ...

While the file <state dir>/hold-<port> exists, nothing is forwarded either way on the connections
through <port>, old or new; they are accepted and kept open, as a replica that is slow or stuck
behind the network would. Removing the file lets everything through again.
"""
import asyncio
import os
import sys


async def pipe(reader, writer, held):
    try:
        while True:
            data = await reader.read(65536)
            if not data:
                break
            while os.path.exists(held):
                await asyncio.sleep(0.05)
            writer.write(data)
            await writer.drain()
    except (ConnectionError, asyncio.CancelledError):
        pass
    finally:
        writer.close()


async def serve(state, ip, port, target_ip, target_port):
    held = os.path.join(state, "hold-%d" % port)

    async def handle(reader, writer):
        try:
            up_reader, up_writer = await asyncio.open_connection(target_ip, target_port)
        except OSError:
            writer.close()
            return
        await asyncio.gather(pipe(reader, up_writer, held), pipe(up_reader, writer, held))

    server = await asyncio.start_server(handle, ip, port)
    async with server:
        await server.serve_forever()


async def main():
    state, ip = sys.argv[1], sys.argv[2]
    tasks = []
    for arg in sys.argv[3:]:
        port, target = arg.split("=")
        target_ip, target_port = target.split(":")
        tasks.append(serve(state, ip, int(port), target_ip, int(target_port)))
    await asyncio.gather(*tasks)


asyncio.run(main())
