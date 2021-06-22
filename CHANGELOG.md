# 0.1.2

* Replace metadata public key context with with a peer context.

  **Extracting a public key**
  ```
    // Previously
	pubKey, ok := metadata.PublicKeyFromContext(ctx)
	if !ok {
		return nil, errors.New("could not extract public key")
	}

    // Now
    p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, errors.New("could not extract peer information")
	}
    pubKey := p.PublicKey
  ```

  **Making a server side RPC call**
  ```
  // Previously
  ctx := context.WithValue(context.Background(), metadata.PublicKeyCtxKey, pubKey)
  res, err := c.Gnip(ctx, &pb.GnipRequest{Body: "Gnip"})

  // Now
  ctx := peer.NewCallContext(context.Background(), pubKey)
  res, err := c.Gnip(ctx, &pb.GnipRequest{Body: "Gnip"})
  ```

# 0.1.1

## Changed

* Supress logging until we can implement a configurable logging solution.

# 0.1.0

Initial release