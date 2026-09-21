# Move Validation & DOP Architecture

## Goal

Prevent invalid moves from being executed or unnecessarily processed. The engine should generate and accept only legal moves whenever possible.

## Architecture

```text
INPUT
  │
  ▼
Cheap Validation
  ├─ valid source/destination
  ├─ piece exists
  ├─ correct side to move
  ├─ destination is not occupied by own piece
  └─ piece movement geometry
  │
  ▼
Special Rules
  ├─ castling
  ├─ en passant
  ├─ promotion
  └─ pawn double move
  │
  ▼
King Safety
  └─ move must not leave own king in check
  │
  ▼
MAKE MOVE
```

## Move Generation

Prefer generating only valid destinations instead of generating arbitrary moves and rejecting them later.

```text
generate_legal_moves(state)
        ↓
legal moves only
```

For UI/input:

```text
legal_destinations(state, source)
        ↓
only selectable squares
```

This prevents invalid moves from even reaching the state mutation layer.

## DOP Principles

Keep data separate from behavior.

### Data

```text
board
turn
castling
en_passant
halfmove
fullmove
history
```

Use contiguous primitive data where possible:

```text
board = bytearray(64)
```

Pieces can be represented by small integer constants.

### Systems

```text
generate_moves()
is_attacked()
is_in_check()
validate_move()
make_move()
undo_move()
```

`is_attacked()` should directly inspect the board instead of generating moves. This avoids unnecessary recursion and keeps the legality system fast.

## Validation vs State Mutation

`make_move()` should **not** validate moves.

Validation happens first:

```python
if can_move(state, move):
    make_move(state, move)
```

Once a move is validated, `make_move()` can assume it is legal and perform only the required memory changes.

This keeps the hot path small and predictable.

## Legal Move Pipeline

```text
                DATA
                  │
                  ▼
            MOVE GENERATOR
                  │
                  ▼
         Pseudo-legal moves
                  │
                  ▼
             VALIDATION
                  │
          ┌───────┴────────┐
          │                │
     Special rules     King safety
          │                │
          └───────┬────────┘
                  ▼
             Legal moves
                  │
                  ▼
               MAKE
                  │
                  ▼
               UNDO
```

## Core Invariant

```text
Invalid moves:
    never reach make_move()

Valid moves:
    make_move() can execute directly

Failed validation:
    state remains unchanged
```

## Compact Representation

Target compact structures:

```text
Move  → 32-bit integer
Board → 64 bytes during execution
Undo  → compact fixed-size data
```

The board can later be packed to **4 bits per square (32 bytes)** for storage, serialization, or hashing if required.

The primary goal is to keep the runtime representation simple and fast while maintaining a data-oriented architecture.
