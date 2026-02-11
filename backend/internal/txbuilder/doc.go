package txbuilder

// Usage example (not compiled):
//
//  auto, err := txbuilder.NewAutoBuilderFromConfig(client, cfg)
//  if err != nil { ... }
//  auto.Start(ctx) // background fee refresh
//
//  tx, err := auto.BuildBuyTx(ctx, from, pair, ethInWei, minTokensOut)
//  // sign + send tx
//
