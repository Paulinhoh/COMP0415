def fibonacci(n):
    if n <= 0:
        return []
    elif n == 1:
        return [0]
    elif n == 2:
        return [0, 1]
    else:
        sequence = fibonacci(n - 1)
        next_number = sequence[-1] + sequence[-2]
        sequence.append(next_number)
        return sequence


def main():
    print(fibonacci(6))


if __name__ == '__main__':
    main() 