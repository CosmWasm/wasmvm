(module
  (memory 1 2)
  (export "execute" (func $execute))
  (func $execute (param i32) (result i32)
    (drop (memory.grow (i32.const 3)))
    (i32.const 0)
  )
)