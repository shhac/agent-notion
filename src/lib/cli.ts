import {
  Command,
  ref,
  type Command as VipvotCommand,
  type ArgsValidator,
} from "vipvot";

export type StringOpt<R extends boolean = boolean> = {
  type: "string";
  short?: string;
  default?: string;
  description: string;
  required?: R;
};

export type BoolOpt = {
  type: "bool";
  short?: string;
  default?: boolean;
  description: string;
};

export type Opt = StringOpt | BoolOpt;

type StringValue<O extends StringOpt> = O extends StringOpt<true>
  ? string
  : O extends { default: string }
    ? string
    : string | undefined;

type OptValue<O extends Opt> = O extends BoolOpt
  ? boolean
  : O extends StringOpt
    ? StringValue<O>
    : never;

type OptsOf<S extends Record<string, Opt> | undefined> = S extends Record<
  string,
  Opt
>
  ? { [K in keyof S]: OptValue<S[K]> }
  : Record<string, never>;

export type DefineCommandSpec<S extends Record<string, Opt> | undefined> = {
  use: string;
  short?: string;
  long?: string;
  args?: ArgsValidator;
  options?: S;
  action: (positional: string[], opts: OptsOf<S>) => unknown | Promise<unknown>;
};

function camelToKebab(name: string): string {
  return name.replace(/[A-Z]/g, (c) => `-${c.toLowerCase()}`);
}

type RegisteredFlag = {
  optKey: string;
  flagName: string;
  type: "string" | "bool";
  hasDefault: boolean;
  ref: { value: unknown };
};

export function defineCommand<S extends Record<string, Opt> | undefined>(
  spec: DefineCommandSpec<S>,
): VipvotCommand {
  const registered: RegisteredFlag[] = [];

  const cmd = Command({
    use: spec.use,
    short: spec.short,
    long: spec.long,
    args: spec.args,
    runE: async (c, args) => {
      const opts: Record<string, unknown> = {};
      const flagSet = c.flags();
      for (const f of registered) {
        if (f.type === "string" && !f.hasDefault) {
          const def = flagSet.byName.get(f.flagName);
          if (!def?.changed) {
            opts[f.optKey] = undefined;
            continue;
          }
        }
        opts[f.optKey] = f.ref.value;
      }
      return spec.action(args, opts as OptsOf<S>) as
        | Error
        | Promise<Error | undefined>
        | undefined;
    },
  });

  if (spec.options) {
    const flags = cmd.flags();
    for (const [optKey, opt] of Object.entries(spec.options)) {
      const flagName = camelToKebab(optKey);
      if (opt.type === "bool") {
        const defaultValue = opt.default ?? false;
        const r = ref<boolean>(defaultValue);
        flags.boolVarP(r, flagName, opt.short ?? "", defaultValue, opt.description);
        registered.push({
          optKey,
          flagName,
          type: "bool",
          hasDefault: opt.default !== undefined,
          ref: r,
        });
      } else {
        const defaultValue = opt.default ?? "";
        const r = ref<string>(defaultValue);
        flags.stringVarP(r, flagName, opt.short ?? "", defaultValue, opt.description);
        if (opt.required) flags.markFlagRequired(flagName);
        registered.push({
          optKey,
          flagName,
          type: "string",
          hasDefault: opt.default !== undefined,
          ref: r,
        });
      }
    }
  }

  return cmd;
}

export type { VipvotCommand as Command };
export {
  ArbitraryArgs,
  ExactArgs,
  MaximumNArgs,
  MinimumNArgs,
  NoArgs,
  RangeArgs,
} from "vipvot";
