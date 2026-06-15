# ADR 0006: XTS encryption skips plaintext PFS blocks

Date: 2026-06-13

## Status

Accepted

## Context

Outer PFS images use AES-128-XTS with 0x1000-byte sectors. LibOrbisPkg does not encrypt every sector in the image. It leaves the first PFS block plaintext so readers can parse the header before deriving decryption state.

Signed outer PFS images also contain an empty block after the flat path table. LibOrbisPkg's sector generator skips that whole block during XTS encryption.

## Decision

PKG Forge leaves sectors 0-15 plaintext and starts XTS encryption at `BlockSize / 0x1000`. For signed outer PFS images, it also skips the empty block after the flat path table.

## Consequences

The outer PFS matches LibOrbisPkg sector encryption behavior. This keeps package size and encrypted block layout stable when comparing generated PKGs against PkgTool.Core output.

The encryption function now accepts an optional block skip map. Callers that do not build signed outer PFS images can keep using the default XTS path.

## Alternatives Considered

### Encrypt every sector after the header

This passed some extraction paths because the empty block contains zeroes, but it did not match LibOrbisPkg and left a hidden format difference in signed outer PFS images.

### Special-case the skip inside the PFS builder only

That would keep the crypto API smaller, but it would hide an XTS rule inside PFS assembly. Passing explicit skipped blocks keeps the behavior visible at the encryption boundary.
