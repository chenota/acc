(in-package :acc/test)

(fiveam:def-suite token-sequence)
(fiveam:in-suite token-sequence)

(fiveam:def-fixture
    token-sequence-test-env ()
  (let* ((raw-seq-len 10)
         (raw-seq
          (loop
         for i from 0 below raw-seq-len
         collect (acc::make-token :kind :test-token :value i :loc (list 0 0) :len 0))))
    (&body)))

(fiveam:test test-initialization
  (fiveam:with-fixture token-sequence-test-env ()
    (fiveam:is (acc::make-token-sequence raw-seq))
    (fiveam:is (acc::make-token-sequence (coerce raw-seq 'vector)))
    (fiveam:is (acc::make-token-sequence nil))
    (fiveam:signals error (acc::make-token-sequence '("cat" "dog")))
    (fiveam:signals error (acc::make-token-sequence 100))))

(fiveam:test test-peek
  (fiveam:with-fixture token-sequence-test-env ()
    (fiveam:is (= 0 (acc::token-value (acc::peek (acc::make-token-sequence raw-seq)))))
    (fiveam:is (eq :ENDMARKER (acc::token-kind (acc::peek (acc::make-token-sequence nil)))))))

(fiveam:test test-advance
  (fiveam:with-fixture token-sequence-test-env ()
    (fiveam:is (= 0 (acc::token-value (acc::advance (acc::make-token-sequence raw-seq)))))
    (fiveam:is (= 1 (acc::token-value (let ((s (acc::make-token-sequence raw-seq))) (acc::advance s) (acc::advance s)))))
    (fiveam:is (eq :ENDMARKER (acc::token-kind (let ((s (acc::make-token-sequence raw-seq))) (dotimes (i raw-seq-len) (acc::advance s)) (acc::advance s)))))))

(fiveam:test test-capture-restore
  (fiveam:with-fixture token-sequence-test-env ()
    (fiveam:is (= 0 (acc::capture (acc::make-token-sequence raw-seq))))
    (fiveam:is (= 1 (let ((s (acc::make-token-sequence raw-seq))) (acc::advance s) (acc::capture s))))
    (fiveam:is (= raw-seq-len (let ((s (acc::make-token-sequence raw-seq))) (acc::restore s raw-seq-len) (acc::capture s))))
    (fiveam:signals error (acc::restore (acc::make-token-sequence raw-seq) (1+ raw-seq-len)))))

(fiveam:test test-expect
  (fiveam:with-fixture token-sequence-test-env ()
    (fiveam:is (not (null (acc::expect (acc::make-token-sequence raw-seq) :test-token))))
    (fiveam:is (null (acc::expect (acc::make-token-sequence raw-seq) :not-a-real-token)))
    (fiveam:is (= 1 (acc::token-value (let ((s (acc::make-token-sequence raw-seq))) (acc::advance s) (acc::expect s :test-token)))))))

(fiveam:test test-expect-with-value
  (fiveam:with-fixture token-sequence-test-env ()
    (fiveam:is (not (null (acc::expect-with-value (acc::make-token-sequence raw-seq) :test-token 0))))
    (fiveam:is (not (null (let ((s (acc::make-token-sequence raw-seq))) (acc::advance s) (acc::expect-with-value s :test-token 1)))))
    (fiveam:is (null (acc::expect-with-value (acc::make-token-sequence raw-seq) :not-a-real-token 0)))
    (fiveam:is (null (acc::expect-with-value (acc::make-token-sequence raw-seq) :test-token "burger")))))