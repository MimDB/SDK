import type {
  SubscribeOptions,
  SubscriptionHandle,
  SubscriptionStatus,
  SubscribeMessage,
  ServerEventMessage,
  ServerErrorMessage,
  RealtimeEvent,
} from './types'

/**
 * Represents a single table subscription mapped 1:1 to a server-side
 * subscription identified by `id`.
 *
 * Manages a status state machine (`pending` -> `active` -> `closed` | `error`)
 * and dispatches incoming server messages to the caller-provided callbacks.
 *
 * @template T - Shape of the row data for typed `new` field on events.
 *
 * @example
 * ```ts
 * const sub = new Subscription('sub-1', 'users', { onEvent: console.log }, ws.send.bind(ws))
 * ws.send(JSON.stringify(sub.buildSubscribeMessage()))
 * // later...
 * sub.unsubscribe()
 * ```
 */
export class Subscription<T = Record<string, unknown>> implements SubscriptionHandle {
  /** Internal subscription ID matching the server-side registration. */
  readonly id: string

  /** The subscribed table name. */
  readonly table: string

  private _status: SubscriptionStatus = 'pending'
  private readonly options: SubscribeOptions<T>
  private readonly sendFn: (data: string) => void

  /**
   * @param id - Unique subscription identifier (e.g. `'sub-1'`).
   * @param table - Database table to subscribe to.
   * @param options - Event callbacks and optional filter/event settings.
   * @param sendFn - Function used to write raw JSON strings to the transport.
   */
  constructor(
    id: string,
    table: string,
    options: SubscribeOptions<T>,
    sendFn: (data: string) => void,
  ) {
    this.id = id
    this.table = table
    this.options = options
    this.sendFn = sendFn
  }

  /**
   * Current state of this subscription.
   *
   * State machine:
   * - `pending`  - created, subscribe message not yet acknowledged
   * - `active`   - server confirmed subscription
   * - `error`    - server returned an error; no further events will arrive
   * - `closed`   - unsubscribed by the client
   */
  get status(): SubscriptionStatus {
    return this._status
  }

  /**
   * Build the subscribe message to send to the server.
   * Defaults `event` to `'*'` when not specified in options.
   *
   * @returns A `SubscribeMessage` ready to be JSON-serialised and sent.
   */
  buildSubscribeMessage(): SubscribeMessage {
    const msg: SubscribeMessage = {
      type: 'subscribe',
      id: this.id,
      table: this.table,
      event: this.options.event ?? '*',
    }
    if (this.options.filter) {
      msg.filter = this.options.filter
    }
    return msg
  }

  /**
   * Handle server confirmation that this subscription is now active.
   * Transitions status to `active` and invokes `onSubscribed` if provided.
   */
  handleSubscribed(): void {
    this._status = 'active'
    this.options.onSubscribed?.()
  }

  /**
   * Handle an incoming event message from the server.
   * No-ops if the subscription is `closed` (guards against late delivery).
   *
   * @param msg - The raw server event message.
   */
  handleEvent(msg: ServerEventMessage): void {
    if (this._status === 'closed') return
    const event: RealtimeEvent<T> = {
      type: msg.event,
      table: msg.table,
      new: msg.new as T | null,
      old: msg.old,
    }
    this.options.onEvent(event)
  }

  /**
   * Handle a server error targeting this subscription.
   * Transitions status to `error` and invokes `onError` if provided.
   *
   * @param msg - The raw server error message.
   */
  handleError(msg: ServerErrorMessage): void {
    this._status = 'error'
    this.options.onError?.({ code: msg.error_code, message: msg.message })
  }

  /**
   * Unsubscribe from the server and mark this subscription as closed.
   * Sends an `unsubscribe` message over the transport.
   * Idempotent: subsequent calls after the first are no-ops.
   */
  unsubscribe(): void {
    if (this._status === 'closed') return
    this._status = 'closed'
    this.sendFn(JSON.stringify({ type: 'unsubscribe', id: this.id }))
  }

  /**
   * Reset status to `pending` to allow resubscription after a reconnect.
   * No-ops if the subscription has been explicitly closed by the client.
   */
  resetForReconnect(): void {
    if (this._status === 'closed') return
    this._status = 'pending'
  }
}
