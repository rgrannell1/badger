from setuptools import setup

setup(
    name='badger',
    version='0.1.0',
    author='Róisín Grannell',
    author_email='r.grannell2@gmail.com',

    url='http://pypi.python.org/pypi/PackageName/',
    license='LICENSE.txt',
    description='Badger is a tool that helps you filter large folders of photos',
    long_description=open('README.md').read(),
    install_requires=[
        "numpy",
        "opencv-python",
        "docopt",
        "sklearn",
        "Pillow",
        "alive_progress"
    ],
    packages=['badger'],
    entry_points={
        'console_scripts': ['badger=badger.cli:main']
    }
)
