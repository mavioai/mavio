// The assistant's fast path in the browser (docs/assistant.md §4.3): the
// WASM module of libs/intent, which the server runs too, instantiated from
// its bytes. Requests and answers are protobuf messages of mavio.intent.v1
// passed through the module's memory.
import { create, type DescMessage, fromBinary, type MessageInitShape, type MessageShape, toBinary } from "@bufbuild/protobuf";
import {
  ExamplesRequestSchema,
  ExamplesResponseSchema,
  type Intent,
  LanguagesResponseSchema,
  ParseIntentRequestSchema,
  ParseIntentResponseSchema,
  ReplyRequestSchema,
  ReplyResponseSchema,
  type Slot,
} from "@mavio/proto/mavio/intent/v1/intent_pb";

interface Exports {
  memory: WebAssembly.Memory;
  intent_alloc(len: number): number;
  intent_free(ptr: number, len: number): void;
  intent_parse(ptr: number, len: number): bigint;
  intent_reply(ptr: number, len: number): bigint;
  intent_examples(ptr: number, len: number): bigint;
  intent_languages(): bigint;
}

/** The grammar of libs/intent, loaded once per page. */
export class Grammar {
  private constructor(private readonly x: Exports) {}

  /** Instantiates the module from its bytes, or from a response fetching them. */
  static async load(source: BufferSource | Response | Promise<Response>): Promise<Grammar> {
    const s = await source;
    const { instance } =
      s instanceof Response
        ? await WebAssembly.instantiate(await s.arrayBuffer())
        : await WebAssembly.instantiate(s as BufferSource);
    return new Grammar(instance.exports as unknown as Exports);
  }

  /** The intents a text may mean, best first; none when the grammar does not match it. */
  parse(req: MessageInitShape<typeof ParseIntentRequestSchema>) {
    return this.call(this.x.intent_parse, ParseIntentRequestSchema, req, ParseIntentResponseSchema).intents;
  }

  /** A reply's text in a language, with its arguments. */
  reply(language: string, key: string, args: Record<string, string> = {}): string {
    return this.call(this.x.intent_reply, ReplyRequestSchema, { language, key, args }, ReplyResponseSchema).text;
  }

  /** Examples of what can be said in a language. */
  examples(language: string): string[] {
    return this.call(this.x.intent_examples, ExamplesRequestSchema, { language }, ExamplesResponseSchema).examples;
  }

  /** The languages the grammar has, as BCP 47 tags. */
  languages(): string[] {
    return this.take(this.x.intent_languages(), LanguagesResponseSchema).languages;
  }

  private call<I extends DescMessage, O extends DescMessage>(
    f: (ptr: number, len: number) => bigint,
    in_: I,
    req: MessageInitShape<I>,
    out: O,
  ): MessageShape<O> {
    const bytes = toBinary(in_, create(in_, req));
    const ptr = bytes.length > 0 ? this.x.intent_alloc(bytes.length) : 0;
    if (bytes.length > 0) new Uint8Array(this.x.memory.buffer, ptr, bytes.length).set(bytes);
    try {
      return this.take(f(ptr, bytes.length), out);
    } finally {
      if (bytes.length > 0) this.x.intent_free(ptr, bytes.length);
    }
  }

  /** Reads and frees an answer: its address in the high 32 bits, its length in the low ones. */
  private take<O extends DescMessage>(r: bigint, out: O): MessageShape<O> {
    if (r === 0n) throw new Error("intent: the module could not read the request");
    const ptr = Number(r >> 32n);
    const len = Number(r & 0xffff_ffffn);
    try {
      return fromBinary(out, new Uint8Array(this.x.memory.buffer, ptr, len).slice());
    } finally {
      if (len > 0) this.x.intent_free(ptr, len);
    }
  }
}

/** An intent's slot by name. */
export function slotOf(intent: Intent, name: string): Slot | undefined {
  return intent.slots.find((s) => s.name === name);
}

/** A slot's number: a number, a duration's seconds or an ordinal. */
export function slotNumber(intent: Intent, name: string): number | undefined {
  const v = slotOf(intent, name)?.value;
  return v?.case === "number" || v?.case === "seconds" || v?.case === "ordinal" ? v.value : undefined;
}

/** A slot's keyword, such as "up" or "intro". */
export function slotKeyword(intent: Intent, name: string): string | undefined {
  const v = slotOf(intent, name)?.value;
  return v?.case === "keyword" ? v.value : undefined;
}

/** A slot's language, as BCP 47. */
export function slotLanguage(intent: Intent, name: string): string | undefined {
  const v = slotOf(intent, name)?.value;
  return v?.case === "language" ? v.value : undefined;
}
