export type Absent<T, K extends keyof T> = { [k in Exclude<keyof T, K>]?: undefined };
export type OneOf<T> =
  | { [k in keyof T]?: undefined }
  | (
    keyof T extends infer K ?
      (K extends string & keyof T ? { [k in K]: T[K] } & Absent<T, K>
        : never)
    : never);