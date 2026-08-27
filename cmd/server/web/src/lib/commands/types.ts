export interface Command {
  label(): string
  apply(): Promise<void>
  revert(): Promise<void>
  /** Undo optimistic store patches when apply() fails. No API calls. */
  rollback?(): void
}
