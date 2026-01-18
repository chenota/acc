(in-package :acc)

(with-ignore-coverage
  (define-condition location-error (error)
      ((location :initarg :location
                 :reader location-error-location
                 :documentation "Format (row col)")
       (message :initarg :message
                :reader location-error-message
                :documentation "Error description"))
    (:report (lambda (condition stream)
               (let ((loc (location-error-location condition)))
                 (format stream "Error at ~A,~A: ~A"
                   (first loc)
                   (second loc)
                   (location-error-message condition))))))

  (define-condition parse-type-error (location-error) ()))