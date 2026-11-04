import argparse
import json
import os
import sqlite3
import sys
from examples.checkout.ingest_client import Client, IngestError
from .app import create_order, dispatch_once, status

def main():
    parser = argparse.ArgumentParser(description="One-shot transactional outbox example")
    commands = parser.add_subparsers(dest="command", required=True)
    create = commands.add_parser("create-order")
    create.add_argument("--database", required=True)
    create.add_argument("--order-id", required=True)
    create.add_argument("--amount-cents", type=int, required=True)
    dispatch = commands.add_parser("dispatch")
    dispatch.add_argument("--database", required=True)
    dispatch.add_argument("--batch-size", type=int, default=100)
    dispatch.add_argument("--max-attempts", type=int, default=3)
    inspect = commands.add_parser("status")
    inspect.add_argument("--database", required=True)
    args = parser.parse_args()
    try:
        if args.command == "create-order":
            result = {"event_id":create_order(args.database,args.order_id,args.amount_cents)}
        elif args.command == "status":
            result = status(args.database)
        else:
            client = Client(os.environ["API_URL"],os.environ["INGESTKIT_API_KEY"],
                            max_attempts=args.max_attempts,ca_file=os.environ.get("INGESTKIT_CA_FILE"))
            result = dispatch_once(args.database,client,args.batch_size)
        print(json.dumps(result))
        return 1 if result.get("error") else 0
    except IngestError as error:
        print(json.dumps({"error":str(error)}))
    except (ValueError,KeyError,OSError,sqlite3.Error):
        print(json.dumps({"error":"invalid outbox input, database or configuration"}))
    return 1

if __name__ == "__main__":
    sys.exit(main())
