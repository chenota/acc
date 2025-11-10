(in-package :acc)

(fiveam:def-suite token-sequence)

(def-fixture
    token-sequence-test-env ()
  (let* ((raw-seq
          (loop
         for i from 0 below 10
         collect (make-token :kind :test-token :value i :row 0 :col 0 :len 0))))
    (&body)))

(test test-initialization
  (with-fixture token-sequence-test-env ()
    (is (make-token-sequence raw-seq))
    (is (make-token-sequence (coerce raw-seq 'vector)))
    (is (make-token-sequence nil))
    (signals error (make-token-sequence '("cat" "dog")))
    (signals error (make-token-sequence 100))))

(test test-peek
  (with-fixture token-sequence-test-env ()
    (is (= 0 (token-value (peek (make-token-sequence raw-seq)))))
    (signals error (peek (make-token-sequence nil)))))

(test test-advance
  (with-fixture token-sequence-test-env ()
    (is (= 0 (token-value (advance (make-token-sequence raw-seq)))))
    (is (= 1 (let ((s (make-token-sequence raw-seq))) (token-value (progn (advance s) (advance s))))))
    (signals error (let ((s (make-token-sequence raw-seq))) (dotimes (i 11) (advance s))))))