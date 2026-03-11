/* global db */

function createIndexWithLog(collectionName, keys, options) {
  const collection = db.getCollection(collectionName);
  const name = collection.createIndex(keys, options || {});
  print(`[OK] ${collectionName}.${name}`);
}

print(`[INFO] applying indexes on db=${db.getName()}`);

createIndexWithLog(
  "Transaction",
  { blocktime: -1 },
  { name: "idx_transaction_blocktime_desc" }
);

createIndexWithLog(
  "TransferNotification",
  { from: 1, to: 1 },
  { name: "idx_transfernotification_from_to" }
);

createIndexWithLog(
  "Nep11TransferNotification",
  { from: 1, to: 1 },
  { name: "idx_nep11transfernotification_from_to" }
);

createIndexWithLog(
  "Market",
  { asset: 1, market: 1, amount: 1, tokenid: 1 },
  { name: "idx_market_asset_market_amount_tokenid" }
);



print("[DONE] index apply completed");
