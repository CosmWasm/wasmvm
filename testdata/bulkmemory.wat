(module
  (memory 1)
  (export "execute" (func $execute))
  (func $execute (param i32) (result i32)
    (memory.copy (i32.const 0) (i32.const 65535) (i32.const 10))
    (i32.const 0)
  )
)