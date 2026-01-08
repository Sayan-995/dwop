fun a():
    print("hello from a")

fun b(x:a):
    v = open("x", "r", encoding="utf-8").read().strip()
    print(v + " -> b")

fun c(y:b):
    import time
    time.sleep(1)
    v = open("y", "r", encoding="utf-8").read().strip()
    print(v + " -> c")
