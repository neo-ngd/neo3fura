/* global db */

function printIndexes(collectionName) {
  const collection = db.getCollection(collectionName);
  const indexes = collection.getIndexes().map((idx) => idx.name);
  print(`${collectionName}: ${indexes.join(", ")}`);
}

print(`[INFO] verifying indexes on db=${db.getName()}`);

[
  "Transaction",
  "Address",
  "Address-Asset",
  "TransferNotification",
  "Nep11TransferNotification",
  "Market",
  "Nep11Properties",
].forEach(printIndexes);

print("[DONE] index verify completed");
