(module
  (memory 1)
  (export "execute" (func $execute))
  (func $execute (param i32) (result i32)
    (i32.store (i32.const 65536) (i32.const 42))
    (i32.const 0)
  )
)