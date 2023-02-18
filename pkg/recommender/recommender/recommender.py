#!/usr/bin/python3

import numpy as np
from sklearn.experimental import enable_iterative_imputer
from sklearn.impute import IterativeImputer
import pandas as pd

from scipy.linalg import sqrtm


class Imputer:
    def __init__(self):
        self.imp = None

    def fit(self, user_item_matrix):
        imp = IterativeImputer(max_iter=10, random_state=0)
        imp.fit(user_item_matrix)
        IterativeImputer(random_state=0)

        self.imp = imp

        return imp

    def predict(self, test_set):
        full_matrix = np.array(self.imp.transform(test_set))

        return full_matrix
